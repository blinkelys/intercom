package speech

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"procom/internal/audio"
	"procom/internal/channels"
	"procom/internal/config"
	"procom/internal/events"
)

// EngineFactory builds a speech engine for the provided speech configuration.
type EngineFactory func(config.SpeechConfig, *log.Logger) (Engine, error)

// Manager bridges audio events into the configured speech engine and republishes results.
type Manager struct {
	config config.Config
	bus    *events.Bus
	logger *log.Logger
	build  EngineFactory

	channelLanguage map[string]string
	channelName     map[string]string
	recentContext   map[string]string
	keywordHints    string
	diagnostics     Diagnostics

	sentenceMinWords          int
	sentenceMinPunctuated     int
	sentenceMaxBufferDuration time.Duration
	minChunkRMS               float64
	minChunkPeak              float64
	vadFloorMin               float64
	vadFloorMultiplier        float64
	vadNoiseUpdateAlpha       float64
	minFinalChars             int

	mu           sync.Mutex
	engine       Engine
	subscription *events.Subscription
	started      bool
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// Diagnostics captures speech runtime metadata for operator visibility.
type Diagnostics struct {
	Model            string            `json:"model"`
	Task             string            `json:"task"`
	Temperature      float64           `json:"temperature"`
	BestOf           int               `json:"bestOf"`
	BeamSize         int               `json:"beamSize"`
	ChannelLanguages map[string]string `json:"channelLanguages"`
	LastChannelID    string            `json:"lastChannelId"`
	LastLanguage     string            `json:"lastLanguage"`
	LastInferenceMS  int               `json:"lastInferenceMs"`
	LastTextChars    int               `json:"lastTextChars"`
	LastError        string            `json:"lastError"`
	UpdatedAt        time.Time         `json:"updatedAt"`
}

type bufferedSentence struct {
	text      string
	firstSeen time.Time
	latest    Result
}

const (
	sentenceFlushTick      = 200 * time.Millisecond
	defaultSentenceMaxWait = 850 * time.Millisecond
	defaultMinWords        = 4
	defaultMinPunctWords   = 2
	defaultMinChunkRMS     = 0.00012
	defaultMinChunkPeak    = 0.003
	defaultVADFloorMin     = 0.00006
	defaultVADFloorMul     = 2.8
	defaultVADNoiseAlpha   = 0.08
	defaultMinFinalChars   = 2
	maxPromptChars         = 520
	maxContextChars        = 160
	maxKeywordHints        = 12
)

// NewManager constructs a lifecycle-managed speech subsystem.
func NewManager(cfg config.Config, bus *events.Bus, logger *log.Logger, build EngineFactory) (*Manager, error) {
	if bus == nil {
		return nil, fmt.Errorf("event bus is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}
	channelLanguage := buildChannelLanguageMap(cfg.Channels)
	channelName := buildChannelNameMap(cfg.Channels)
	diagnosticsChannels := make(map[string]string, len(channelLanguage))
	for key, value := range channelLanguage {
		diagnosticsChannels[key] = value
	}
	keywordHints := buildKeywordHints(cfg.Keywords)

	return &Manager{
		config:                    cfg,
		bus:                       bus,
		logger:                    logger,
		build:                     build,
		channelLanguage:           channelLanguage,
		channelName:               channelName,
		recentContext:             make(map[string]string, len(channelLanguage)),
		keywordHints:              keywordHints,
		sentenceMinWords:          getenvIntDefault("PROCOM_SENTENCE_MIN_WORDS", defaultMinWords),
		sentenceMinPunctuated:     getenvIntDefault("PROCOM_SENTENCE_MIN_PUNCT_WORDS", defaultMinPunctWords),
		sentenceMaxBufferDuration: time.Duration(getenvIntDefault("PROCOM_SENTENCE_MAX_WAIT_MS", int(defaultSentenceMaxWait/time.Millisecond))) * time.Millisecond,
		minChunkRMS:               getenvFloatDefault("PROCOM_AUDIO_MIN_RMS", defaultMinChunkRMS),
		minChunkPeak:              getenvFloatDefault("PROCOM_AUDIO_MIN_PEAK", defaultMinChunkPeak),
		vadFloorMin:               getenvFloatDefault("PROCOM_AUDIO_VAD_FLOOR_MIN", defaultVADFloorMin),
		vadFloorMultiplier:        getenvFloatDefault("PROCOM_AUDIO_VAD_FLOOR_MUL", defaultVADFloorMul),
		vadNoiseUpdateAlpha:       getenvFloatDefault("PROCOM_AUDIO_VAD_NOISE_ALPHA", defaultVADNoiseAlpha),
		minFinalChars:             getenvIntDefault("PROCOM_MIN_FINAL_CHARS", defaultMinFinalChars),
		diagnostics: Diagnostics{
			Model:            getenvDefault("PROCOM_MLX_MODEL", "mlx-community/whisper-tiny"),
			Task:             getenvDefault("PROCOM_MLX_TASK", "transcribe"),
			Temperature:      getenvFloatDefault("PROCOM_MLX_TEMPERATURE", 0),
			BestOf:           getenvIntDefault("PROCOM_MLX_BEST_OF", 1),
			BeamSize:         getenvIntDefault("PROCOM_MLX_BEAM_SIZE", 1),
			ChannelLanguages: diagnosticsChannels,
			UpdatedAt:        time.Now().UTC(),
		},
	}, nil
}

// Name returns the lifecycle component name.
func (m *Manager) Name() string {
	return "speech-manager"
}

// Diagnostics returns a snapshot of speech runtime diagnostics.
func (m *Manager) Diagnostics() Diagnostics {
	m.mu.Lock()
	defer m.mu.Unlock()

	copyChannels := make(map[string]string, len(m.diagnostics.ChannelLanguages))
	for key, value := range m.diagnostics.ChannelLanguages {
		copyChannels[key] = value
	}

	copyDiagnostics := m.diagnostics
	copyDiagnostics.ChannelLanguages = copyChannels
	return copyDiagnostics
}

// Start initializes the speech engine and begins consuming audio events.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return fmt.Errorf("speech manager already started")
	}
	runCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.started = true
	m.mu.Unlock()

	if !m.config.Speech.Enabled {
		m.publishStatus(EngineStateDisabled, "speech disabled by configuration")
		return nil
	}

	if m.build == nil {
		return fmt.Errorf("speech engine factory is required when speech is enabled")
	}

	engine, err := m.build(m.config.Speech, m.logger)
	if err != nil {
		return fmt.Errorf("build speech engine: %w", err)
	}

	subscription, err := m.bus.Subscribe()
	if err != nil {
		return fmt.Errorf("subscribe to event bus: %w", err)
	}

	m.publishStatus(EngineStateStarting, "starting speech engine")
	if err := engine.Start(runCtx); err != nil {
		subscription.Close()
		m.publishStatus(EngineStateFailed, err.Error())
		return fmt.Errorf("start speech engine: %w", err)
	}

	m.mu.Lock()
	m.engine = engine
	m.subscription = subscription
	m.mu.Unlock()

	m.wg.Add(3)
	go m.consumeAudioEvents(runCtx, subscription, engine)
	go m.consumeResults(runCtx, engine)
	go m.consumeErrors(runCtx, engine)

	m.publishStatus(EngineStateRunning, "speech engine running")
	return nil
}

// Stop shuts down the speech engine and background consumers.
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return nil
	}
	m.started = false
	engine := m.engine
	subscription := m.subscription
	cancel := m.cancel
	m.engine = nil
	m.subscription = nil
	m.cancel = nil
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if subscription != nil {
		subscription.Close()
	}
	if engine != nil {
		if err := engine.Stop(); err != nil {
			return err
		}
	}

	stopped := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(stopped)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-stopped:
	}

	m.publishStatus(EngineStateStopped, "speech engine stopped")
	return nil
}

func (m *Manager) consumeAudioEvents(ctx context.Context, subscription *events.Subscription, engine Engine) {
	defer m.wg.Done()

	pendingByChannel := make(map[string][]float32)
	pendingRate := make(map[string]int)
	pendingCapturedAt := make(map[string]time.Time)
	noiseFloorByChannel := make(map[string]float64)
	flushTicker := time.NewTicker(250 * time.Millisecond)
	defer flushTicker.Stop()

	flush := func(channelID string) {
		frames := pendingByChannel[channelID]
		if len(frames) == 0 {
			return
		}

		rate := pendingRate[channelID]
		if rate <= 0 {
			rate = 16000
		}

		capturedAt := pendingCapturedAt[channelID]
		if capturedAt.IsZero() {
			capturedAt = time.Now().UTC()
		}

		language := m.channelLanguage[channelID]
		rms := calculateRMS(frames)
		peak := calculatePeak(frames)
		if !m.shouldSubmitChunk(channelID, rms, peak, noiseFloorByChannel) {
			m.logger.Printf("speech manager vad-drop channel=%s frames=%d rms=%.6f peak=%.6f floor=%.6f", channelID, len(frames), rms, peak, noiseFloorByChannel[channelID])
			delete(pendingByChannel, channelID)
			delete(pendingRate, channelID)
			delete(pendingCapturedAt, channelID)
			return
		}

		chunk := AudioChunk{
			ChannelID:  channelID,
			Language:   language,
			Prompt:     m.buildPrompt(channelID),
			SampleRate: rate,
			Frames:     append([]float32(nil), frames...),
			CapturedAt: capturedAt,
		}
		m.logger.Printf("speech manager submit channel=%s frames=%d sampleRate=%d language=%s promptChars=%d", channelID, len(frames), rate, language, len(chunk.Prompt))

		if err := engine.Submit(chunk); err != nil {
			if errors.Is(err, errEngineQueueFull) {
				m.logger.Printf("speech manager queue backpressure channel=%s frames=%d sampleRate=%d (dropping batch)", channelID, len(frames), rate)
			} else {
				m.logger.Printf("speech manager submit failed channel=%s frames=%d sampleRate=%d err=%v", channelID, len(frames), rate, err)
				m.publishStatus(EngineStateFailed, fmt.Sprintf("submit audio chunk: %v", err))
			}
		}

		delete(pendingByChannel, channelID)
		delete(pendingRate, channelID)
		delete(pendingCapturedAt, channelID)
	}

	for {
		select {
		case <-ctx.Done():
			for channelID := range pendingByChannel {
				flush(channelID)
			}
			return
		case <-flushTicker.C:
			for channelID := range pendingByChannel {
				flush(channelID)
			}
		case event, ok := <-subscription.Events():
			if !ok {
				for channelID := range pendingByChannel {
					flush(channelID)
				}
				return
			}
			switch event.Type {
			case channels.EventTypeUpdated:
				update, ok := event.Payload.(channels.Update)
				if !ok {
					continue
				}
				m.channelLanguage[update.Channel.ID] = update.Channel.Language
				m.mu.Lock()
				if m.diagnostics.ChannelLanguages == nil {
					m.diagnostics.ChannelLanguages = map[string]string{}
				}
				m.diagnostics.ChannelLanguages[update.Channel.ID] = update.Channel.Language
				m.diagnostics.UpdatedAt = time.Now().UTC()
				m.mu.Unlock()
				m.logger.Printf("speech manager channel language updated channel=%s language=%s", update.Channel.ID, update.Channel.Language)
				continue
			case audio.EventTypeChunkCaptured:
				chunk, ok := event.Payload.(audio.SampleChunk)
				if !ok {
					m.publishStatus(EngineStateFailed, fmt.Sprintf("unexpected audio payload type %T", event.Payload))
					continue
				}

				if chunk.SampleRate <= 0 {
					chunk.SampleRate = 16000
				}

				pendingByChannel[chunk.ChannelID] = append(pendingByChannel[chunk.ChannelID], chunk.Frames...)
				pendingRate[chunk.ChannelID] = chunk.SampleRate
				pendingCapturedAt[chunk.ChannelID] = chunk.CapturedAt

				flushThreshold := chunk.SampleRate / 6 // ~167ms batches for lower recognition latency.
				if flushThreshold < 1024 {
					flushThreshold = 1024
				}

				if len(pendingByChannel[chunk.ChannelID]) >= flushThreshold {
					flush(chunk.ChannelID)
				}
				continue
			default:
				continue
			}
		}
	}
}

func buildChannelLanguageMap(channels []config.ChannelConfig) map[string]string {
	result := make(map[string]string, len(channels))
	for _, channel := range channels {
		result[channel.ID] = channel.Language
	}
	return result
}

func buildChannelNameMap(channels []config.ChannelConfig) map[string]string {
	result := make(map[string]string, len(channels))
	for _, channel := range channels {
		result[channel.ID] = channel.Name
	}
	return result
}

func buildKeywordHints(keywords []config.KeywordConfig) string {
	if len(keywords) == 0 {
		return ""
	}

	hints := make([]string, 0, len(keywords))
	for _, keyword := range keywords {
		phrase := strings.TrimSpace(keyword.Phrase)
		if phrase == "" {
			continue
		}
		hints = append(hints, phrase)
		if len(hints) >= maxKeywordHints {
			break
		}
	}
	if len(hints) == 0 {
		return ""
	}
	return "Production vocabulary: " + strings.Join(hints, ", ") + "."
}

func (m *Manager) buildPrompt(channelID string) string {
	m.mu.Lock()
	contextText := m.recentContext[channelID]
	m.mu.Unlock()
	language := strings.ToLower(strings.TrimSpace(m.channelLanguage[channelID]))
	channelName := strings.ToLower(strings.TrimSpace(m.channelName[channelID]))

	parts := make([]string, 0, 5)
	if language == "no" || language == "nb" || language == "nn" {
		parts = append(parts, "Transkriber ordrett pa norsk. Ikke oversett til engelsk.")
	} else {
		parts = append(parts, "Transcribe exactly in the spoken language. Do not translate.")
	}
	if name := strings.TrimSpace(m.channelName[channelID]); name != "" {
		if language == "no" || language == "nb" || language == "nn" {
			parts = append(parts, "Kildekanal: "+name+".")
		} else {
			parts = append(parts, "Source channel: "+name+".")
		}
	}
	if language != "" {
		parts = append(parts, "Target language: "+languageLabel(language)+".")
	}
	if language == "no" || language == "nb" || language == "nn" {
		parts = append(parts, "Behold korte cue-ord nøyaktig slik de sies. I dette domenet er 'tag' et cue-ord og skal forbli 'tag'.")
	}
	if strings.Contains(channelName, "musical") || strings.Contains(channelName, "director") || strings.Contains(channelName, "md") {
		if language == "no" || language == "nb" || language == "nn" {
			parts = append(parts, "Musical-direction vocabulary: vers, refreng, bridge, intro, outro, takt, takter, opptakt, innsatser, monitor, mer i monitor, mindre i monitor, klikk, piano, gitar, bass, trommer, kor.")
			parts = append(parts, "Keep numbers and counts literal in Norwegian: en, to, tre, fire, fem, seks, sju, aatte.")
		} else {
			parts = append(parts, "Musical-direction vocabulary: verse, chorus, bridge, intro, outro, count, bars, pickup, entry, monitor, more in monitor, less in monitor, click, piano, guitar, bass, drums, vocals.")
		}
	}
	if hints := strings.TrimSpace(m.keywordHints); hints != "" {
		parts = append(parts, hints)
	}
	if contextText != "" {
		parts = append(parts, "Recent context: "+contextText)
	}

	prompt := strings.TrimSpace(strings.Join(parts, " "))
	if len(prompt) > maxPromptChars {
		prompt = prompt[len(prompt)-maxPromptChars:]
	}
	return prompt
}

func (m *Manager) rememberContext(channelID string, text string) {
	normalized := strings.Join(strings.Fields(text), " ")
	if normalized == "" {
		return
	}
	if len(normalized) > maxContextChars {
		normalized = normalized[len(normalized)-maxContextChars:]
	}

	m.mu.Lock()
	m.recentContext[channelID] = normalized
	m.mu.Unlock()
}

func calculateRMS(frames []float32) float64 {
	if len(frames) == 0 {
		return 0
	}
	var sum float64
	for _, sample := range frames {
		value := float64(sample)
		sum += value * value
	}
	return sqrt(sum / float64(len(frames)))
}

func calculatePeak(frames []float32) float64 {
	if len(frames) == 0 {
		return 0
	}
	peak := 0.0
	for _, sample := range frames {
		value := float64(sample)
		if value < 0 {
			value = -value
		}
		if value > peak {
			peak = value
		}
	}
	return peak
}

func (m *Manager) shouldSubmitChunk(channelID string, rms float64, peak float64, floorByChannel map[string]float64) bool {
	floor := floorByChannel[channelID]
	if floor == 0 {
		floor = m.vadFloorMin
	}

	if rms < floor {
		floor = ((1 - m.vadNoiseUpdateAlpha) * floor) + (m.vadNoiseUpdateAlpha * rms)
		if floor < m.vadFloorMin {
			floor = m.vadFloorMin
		}
		floorByChannel[channelID] = floor
	} else {
		floorByChannel[channelID] = floor
	}

	dynamicThreshold := floor * m.vadFloorMultiplier
	if dynamicThreshold < m.minChunkRMS {
		dynamicThreshold = m.minChunkRMS
	}

	if rms < dynamicThreshold && peak < m.minChunkPeak {
		return false
	}

	if rms > floor {
		floorByChannel[channelID] = floor + ((rms-floor)*0.02)
	}
	return true
}

func sqrt(value float64) float64 {
	if value <= 0 {
		return 0
	}
	guess := value
	if guess < 1 {
		guess = 1
	}
	for i := 0; i < 8; i++ {
		guess = 0.5 * (guess + value/guess)
	}
	return guess
}

func getenvDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getenvIntDefault(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvFloatDefault(key string, fallback float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func (m *Manager) consumeResults(ctx context.Context, engine Engine) {
	defer m.wg.Done()
	pending := make(map[string]bufferedSentence)
	partialStable := make(map[string]string)
	lastEmitted := make(map[string]string)
	flushTicker := time.NewTicker(sentenceFlushTick)
	defer flushTicker.Stop()

	flush := func(channelID string, force bool) {
		buffered, ok := pending[channelID]
		if !ok {
			return
		}
		if !force {
			clear := m.isClearSentence(buffered.text)
			if !clear && time.Since(buffered.firstSeen) < m.sentenceMaxBufferDuration {
				return
			}
		}

		result := buffered.latest
		result.Text = buffered.text
		result.Final = true
		normalized := normalizeComparable(result.Text)
		if normalized == "" {
			delete(pending, channelID)
			return
		}
		if previous := lastEmitted[channelID]; previous != "" {
			if normalized == previous {
				delete(pending, channelID)
				m.logger.Printf("speech manager suppress duplicate final channel=%s text=%q", channelID, result.Text)
				m.publishPartialClear(channelID)
				return
			}
		}

		result.ReceivedAt = time.Now().UTC()
		delete(pending, channelID)
		delete(partialStable, channelID)
		lastEmitted[channelID] = normalized
		m.rememberContext(channelID, result.Text)
		m.bus.Publish(events.Event{Type: EventTypeResultFinal, Payload: result})
	}

	for {
		select {
		case <-ctx.Done():
			for channelID := range pending {
				flush(channelID, true)
			}
			return
		case <-flushTicker.C:
			for channelID := range pending {
				flush(channelID, false)
			}
		case result, ok := <-engine.Results():
			if !ok {
				for channelID := range pending {
					flush(channelID, true)
				}
				return
			}
			m.logger.Printf("speech result channel=%s final=%t chars=%d", result.ChannelID, result.Final, len(result.Text))
			m.mu.Lock()
			m.diagnostics.LastChannelID = result.ChannelID
			m.diagnostics.LastLanguage = result.Language
			m.diagnostics.LastInferenceMS = result.InferenceMS
			m.diagnostics.LastTextChars = len(result.Text)
			if result.Model != "" {
				m.diagnostics.Model = result.Model
			}
			if result.Task != "" {
				m.diagnostics.Task = result.Task
			}
			m.diagnostics.LastError = ""
			m.diagnostics.UpdatedAt = time.Now().UTC()
			m.mu.Unlock()
			if result.ReceivedAt.IsZero() {
				result.ReceivedAt = time.Now().UTC()
			}

			if !result.Final {
				stable := stabilizePartialText(partialStable[result.ChannelID], result.Text)
				partialStable[result.ChannelID] = stable
				result.Text = stable
				m.rememberContext(result.ChannelID, stable)
				m.bus.Publish(events.Event{Type: EventTypeResultPartial, Payload: result})
				continue
			}
			if m.isLikelyNoiseFinal(result.Text) {
				m.logger.Printf("speech manager drop low-signal final channel=%s text=%q", result.ChannelID, strings.TrimSpace(result.Text))
				m.publishPartialClear(result.ChannelID)
				continue
			}

			combined := mergeSentenceText(pending[result.ChannelID].text, result.Text)
			if combined == "" {
				m.logger.Printf("speech manager drop empty-merge final channel=%s text=%q", result.ChannelID, strings.TrimSpace(result.Text))
				m.publishPartialClear(result.ChannelID)
				continue
			}

			current := pending[result.ChannelID]
			current.text = combined
			if current.firstSeen.IsZero() {
				current.firstSeen = result.ReceivedAt
			}
			current.latest = result
			pending[result.ChannelID] = current

			if m.isClearSentence(current.text) {
				flush(result.ChannelID, false)
				continue
			}

			preview := result
			preview.Final = false
			preview.Text = current.text
			partialStable[result.ChannelID] = current.text
			m.bus.Publish(events.Event{Type: EventTypeResultPartial, Payload: preview})
		}
	}
}

func (m *Manager) publishPartialClear(channelID string) {
	m.bus.Publish(events.Event{Type: EventTypeResultPartial, Payload: Result{
		ChannelID:  channelID,
		Text:       "",
		Final:      false,
		ReceivedAt: time.Now().UTC(),
	}})
}

func mergeSentenceText(existing string, next string) string {
	left := strings.Join(strings.Fields(existing), " ")
	right := strings.Join(strings.Fields(next), " ")

	if left == "" {
		return right
	}
	if right == "" {
		return left
	}
	if strings.Contains(left, right) {
		return left
	}
	if strings.Contains(right, left) {
		return right
	}
	if strings.HasSuffix(left, right) {
		return left
	}
	if strings.HasPrefix(right, left) {
		return right
	}
	return left + " " + right
}

func stabilizePartialText(previous string, incoming string) string {
	left := strings.Fields(previous)
	right := strings.Fields(incoming)
	if len(right) == 0 {
		return strings.TrimSpace(previous)
	}
	if len(left) == 0 {
		return strings.Join(right, " ")
	}

	limit := len(left)
	if len(right) < limit {
		limit = len(right)
	}
	prefix := 0
	for prefix < limit {
		if !strings.EqualFold(left[prefix], right[prefix]) {
			break
		}
		prefix++
	}

	if prefix == 0 {
		return strings.Join(right, " ")
	}

	locked := left[:prefix]
	tail := right[prefix:]
	if len(tail) == 0 {
		return strings.Join(locked, " ")
	}
	return strings.Join(append(locked, tail...), " ")
}

func (m *Manager) isClearSentence(text string) bool {
	words := len(strings.Fields(text))
	if words >= m.sentenceMinWords {
		return true
	}
	if words >= m.sentenceMinPunctuated && hasSentencePunctuation(text) {
		return true
	}
	return false
}

func (m *Manager) isLikelyNoiseFinal(text string) bool {
	normalized := strings.Join(strings.Fields(text), " ")
	if normalized == "" {
		m.logger.Printf("speech manager noise-final reason=empty")
		return true
	}
	if len(normalized) < m.minFinalChars {
		m.logger.Printf("speech manager noise-final reason=too-short chars=%d threshold=%d text=%q", len(normalized), m.minFinalChars, normalized)
		return true
	}
	return false
}

func normalizeComparable(text string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(text), " "))
	replacer := strings.NewReplacer(".", "", ",", "", "!", "", "?", "", ":", "", ";", "")
	return replacer.Replace(normalized)
}

func hasSentencePunctuation(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	last := trimmed[len(trimmed)-1]
	return last == '.' || last == '!' || last == '?'
}

func languageLabel(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "no", "nb", "nn":
		return "Norwegian"
	case "en":
		return "English"
	default:
		if language == "" {
			return ""
		}
		return language
	}
}

func (m *Manager) consumeErrors(ctx context.Context, engine Engine) {
	defer m.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-engine.Errors():
			if !ok {
				return
			}
			m.logger.Printf("speech error: %v", err)
			m.mu.Lock()
			m.diagnostics.LastError = err.Error()
			m.diagnostics.UpdatedAt = time.Now().UTC()
			m.mu.Unlock()
			m.publishStatus(EngineStateFailed, err.Error())
		}
	}
}

func (m *Manager) publishStatus(state EngineState, message string) {
	m.bus.Publish(events.Event{Type: EventTypeEngineStateChanged, Payload: Status{
		Engine:     m.config.Speech.Engine,
		State:      state,
		OccurredAt: time.Now().UTC(),
		Message:    message,
	}})
}
