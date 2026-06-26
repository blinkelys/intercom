#!/usr/bin/env python3
import json
import os
import re
import sys
import time
from collections import defaultdict

import numpy as np

try:
    import mlx_whisper
except Exception as exc:
    print(json.dumps({"type": "error", "message": f"mlx_whisper import failed: {exc}"}), flush=True)
    sys.exit(1)


MODEL = os.environ.get("PROCOM_MLX_MODEL", "mlx-community/whisper-large-v3-turbo")
DEFAULT_LANGUAGE = os.environ.get("PROCOM_MLX_LANGUAGE", "").strip()
TEMPERATURE = float(os.environ.get("PROCOM_MLX_TEMPERATURE", "0"))
BEST_OF = int(os.environ.get("PROCOM_MLX_BEST_OF", "3"))
BEAM_SIZE = int(os.environ.get("PROCOM_MLX_BEAM_SIZE", "1"))
TASK = "transcribe"
NO_SPEECH_THRESHOLD = float(os.environ.get("PROCOM_MLX_NO_SPEECH_THRESHOLD", "0.45"))
LOGPROB_THRESHOLD = float(os.environ.get("PROCOM_MLX_LOGPROB_THRESHOLD", "-0.7"))
COMPRESSION_RATIO_THRESHOLD = float(os.environ.get("PROCOM_MLX_COMPRESSION_RATIO_THRESHOLD", "2.2"))
TARGET_RMS = float(os.environ.get("PROCOM_MLX_TARGET_RMS", "0.015"))
MAX_GAIN = float(os.environ.get("PROCOM_MLX_MAX_GAIN", "16"))
RAW_RMS_FLOOR = float(os.environ.get("PROCOM_MLX_RAW_RMS_FLOOR", "0.00008"))
CONDITION_ON_PREVIOUS_TEXT = os.environ.get("PROCOM_MLX_CONDITION_ON_PREVIOUS_TEXT", "true").strip().lower() in ("1", "true", "yes", "on")

LANGUAGE_ALIASES = {
    "en": "en",
    "english": "en",
    "no": "no",
    "nb": "no",
    "nn": "nn",
    "norwegian": "no",
    "norsk": "no",
}

PARTIAL_INTERVAL_SECONDS = int(os.environ.get("PROCOM_MLX_PARTIAL_INTERVAL_SECONDS", "1"))
PARTIAL_INTERVAL_MS = int(os.environ.get("PROCOM_MLX_PARTIAL_INTERVAL_MS", "400"))
FINAL_WINDOW_SECONDS = int(os.environ.get("PROCOM_MLX_FINAL_WINDOW_SECONDS", "2"))
MAX_BUFFER_SECONDS = int(os.environ.get("PROCOM_MLX_MAX_BUFFER_SECONDS", "8"))
VAD_RMS_GATE = float(os.environ.get("PROCOM_MLX_VAD_RMS_GATE", "0.00012"))
VAD_PEAK_GATE = float(os.environ.get("PROCOM_MLX_VAD_PEAK_GATE", "0.003"))
TEXT_REWRITE_PAIRS = os.environ.get("PROCOM_TEXT_REWRITE_PAIRS", "")


class ChannelState:
    def __init__(self):
        self.chunks = []
        self.sample_rate = 16000
        self.language = ""
        self.prompt = ""
        self.last_partial_samples = 0

    @property
    def total_samples(self):
        return sum(chunk.size for chunk in self.chunks)


states = defaultdict(ChannelState)


def emit(payload):
    print(json.dumps(payload), flush=True)


def log(message):
    print(f"[procom-worker] {message}", file=sys.stderr, flush=True)


def normalize_language(value):
    key = str(value or "").strip().lower()
    if not key:
        return ""
    return LANGUAGE_ALIASES.get(key, key)


def strip_repeated_speaker_labels(text):
    normalized = str(text or "").strip()
    if not normalized:
        return ""

    # Remove leading diarization-like labels repeated by the model, e.g. "Speaker 1 Speaker 1 ...".
    cleaned = re.sub(r"^(?:speaker\s*\d+\s+){2,}", "", normalized, flags=re.IGNORECASE).strip()
    return cleaned or normalized


def is_repetitive_speaker_loop(text):
    normalized = " ".join(str(text or "").strip().lower().split())
    if not normalized:
        return False

    if re.fullmatch(r"(?:speaker\s*\d+)(?:\s+speaker\s*\d+){3,}", normalized):
        return True

    words = normalized.split()
    if len(words) < 8:
        return False

    # Generic short-loop detector for repetitive junk output.
    for size in (1, 2, 3):
        if len(words) % size != 0:
            continue
        pattern = words[:size]
        if pattern * (len(words) // size) == words:
            return True
    return False


def parse_rewrite_pairs(raw):
    pairs = []
    for part in str(raw or "").split(";;"):
        item = part.strip()
        if not item or "=>" not in item:
            continue
        left, right = item.split("=>", 1)
        left = left.strip()
        right = right.strip()
        if left:
            pairs.append((left, right))
    return pairs


REWRITE_RULES = parse_rewrite_pairs(TEXT_REWRITE_PAIRS)


def postprocess_text(text, language):
    value = " ".join(str(text or "").strip().split())
    if not value:
        return ""

    for left, right in REWRITE_RULES:
        value = re.sub(re.escape(left), right, value, flags=re.IGNORECASE)

    if language in ("nb", "no", "nn"):
        value = re.sub(r"\b(Speaker|speaker)\s*\d+\b", "", value).strip()
        value = re.sub(r"\s{2,}", " ", value)

    return value


def run_transcribe_with_fallback(normalized_audio, kwargs):
    """Retry with greedy decoding when beam search is unavailable in local mlx_whisper."""
    active_kwargs = dict(kwargs)
    for _ in range(3):
        try:
            return mlx_whisper.transcribe(normalized_audio, **active_kwargs), active_kwargs
        except Exception as exc:
            message = str(exc).lower()

            prompt_unsupported = (
                ("unexpected keyword argument" in message or "got an unexpected keyword" in message)
                and "initial_prompt" in message
                and "initial_prompt" in active_kwargs
            )
            if prompt_unsupported:
                log("initial_prompt unsupported by current mlx_whisper build; retrying without prompt")
                active_kwargs.pop("initial_prompt", None)
                continue

            # Generic compatibility fallback for older mlx_whisper signatures.
            if "unexpected keyword argument" in message or "got an unexpected keyword" in message:
                for key in ("compression_ratio_threshold", "logprob_threshold", "no_speech_threshold"):
                    if key in message and key in active_kwargs:
                        log(f"{key} unsupported by current mlx_whisper build; retrying without it")
                        active_kwargs.pop(key, None)
                        break
                else:
                    raise
                continue

            beam_requested = int(active_kwargs.get("beam_size", 1) or 1) > 1
            beam_related_error = (
                "beam search" in message
                or "beam_size" in message
                or "decoder is not yet implemented" in message
                or "decoder not implemented" in message
            )
            if beam_related_error and beam_requested:
                active_kwargs["beam_size"] = 1
                active_kwargs["best_of"] = 1
                log("beam search unsupported by current mlx_whisper build; retrying with greedy decode (beam_size=1, best_of=1)")
                continue

            raise

    raise RuntimeError("unable to transcribe audio after compatibility retries")


def transcribe_channel(channel_id, final):
    state = states.get(channel_id)
    if not state or not state.chunks:
        return

    sample_rate = state.sample_rate
    audio = np.concatenate(state.chunks).astype(np.float32)
    if audio.size == 0:
        return

    if not final:
        # Keep partial inference bounded so the worker stays responsive.
        max_samples = sample_rate * FINAL_WINDOW_SECONDS
        if audio.size > max_samples:
            audio = audio[-max_samples:]

    try:
        # Pass raw float32 audio directly to avoid runtime ffmpeg dependency.
        normalized = np.clip(audio, -1.0, 1.0).astype(np.float32)
        current_rms = float(np.sqrt(np.mean(np.square(normalized)))) if normalized.size > 0 else 0.0
        gain = 1.0
        if current_rms > 0 and TARGET_RMS > 0 and current_rms < TARGET_RMS:
            gain = min(MAX_GAIN, TARGET_RMS / current_rms)
            normalized = np.clip(normalized * gain, -1.0, 1.0).astype(np.float32)
        language = normalize_language(state.language or DEFAULT_LANGUAGE)
        kwargs = {
            "path_or_hf_repo": MODEL,
            "task": TASK,
            "temperature": TEMPERATURE,
            "condition_on_previous_text": CONDITION_ON_PREVIOUS_TEXT,
        }
        # Greedy decoding defaults are safest across mlx_whisper variants.
        if BEST_OF > 1:
            kwargs["best_of"] = BEST_OF
        if BEAM_SIZE > 1:
            kwargs["beam_size"] = BEAM_SIZE
        if language:
            kwargs["language"] = language
        if state.prompt:
            kwargs["initial_prompt"] = state.prompt
        # These thresholds reduce hallucinations and false positives when the source is noisy.
        kwargs["no_speech_threshold"] = NO_SPEECH_THRESHOLD
        kwargs["logprob_threshold"] = LOGPROB_THRESHOLD
        kwargs["compression_ratio_threshold"] = COMPRESSION_RATIO_THRESHOLD
        log(f"infer start channel={channel_id} samples={normalized.size} sample_rate={sample_rate} final={final} language={language or 'auto'} model={MODEL} rms={current_rms:.6f} gain={gain:.2f}")
        started = time.time()
        result, effective_kwargs = run_transcribe_with_fallback(normalized, kwargs)
        elapsed_ms = int((time.time() - started) * 1000)
        text = (result.get("text") or "").strip()
        text = strip_repeated_speaker_labels(text)
        text = postprocess_text(text, language)
        segments = result.get("segments") or []
        avg_logprob = None
        no_speech_prob = None
        compression_ratio = None
        if segments:
            avg_values = [segment.get("avg_logprob") for segment in segments if isinstance(segment, dict) and segment.get("avg_logprob") is not None]
            silence_values = [segment.get("no_speech_prob") for segment in segments if isinstance(segment, dict) and segment.get("no_speech_prob") is not None]
            compression_values = [segment.get("compression_ratio") for segment in segments if isinstance(segment, dict) and segment.get("compression_ratio") is not None]
            if avg_values:
                avg_logprob = float(sum(avg_values) / len(avg_values))
            if silence_values:
                no_speech_prob = float(max(silence_values))
            if compression_values:
                compression_ratio = float(max(compression_values))

        if final and text:
            if is_repetitive_speaker_loop(text):
                log(f"infer filtered repetitive-loop channel={channel_id} text={text!r}")
                return

            low_confidence = avg_logprob is not None and avg_logprob < LOGPROB_THRESHOLD
            likely_silence = no_speech_prob is not None and no_speech_prob > NO_SPEECH_THRESHOLD
            likely_hallucination = compression_ratio is not None and compression_ratio > COMPRESSION_RATIO_THRESHOLD
            too_quiet = current_rms < RAW_RMS_FLOOR

            suspicious_signals = 0
            if low_confidence:
                suspicious_signals += 1
            if likely_silence:
                suspicious_signals += 1
            if likely_hallucination:
                suspicious_signals += 1
            if too_quiet:
                suspicious_signals += 1

            # Fail-open mode: keep transcribing even on weak confidence signals.
            if suspicious_signals >= 3 and len(text) < 24:
                log(
                    "infer low-confidence-accepted channel={} final={} text_len={} signals={} avg_logprob={} no_speech_prob={} compression_ratio={} rms={}".format(
                        channel_id,
                        final,
                        len(text),
                        suspicious_signals,
                        avg_logprob,
                        no_speech_prob,
                        compression_ratio,
                        current_rms,
                    )
                )

        log(f"infer done channel={channel_id} final={final} chars={len(text)} elapsed_ms={elapsed_ms}")
        if text:
            emit({
                "type": "result",
                "channelId": channel_id,
                "language": language,
                "model": MODEL,
                "task": TASK,
                "inferenceMs": elapsed_ms,
                "beamSize": int(effective_kwargs.get("beam_size", BEAM_SIZE)),
                "bestOf": int(effective_kwargs.get("best_of", BEST_OF)),
                "text": text,
                "final": final,
            })
    except Exception as exc:
        log(f"infer error channel={channel_id} final={final} err={exc}")
        emit({"type": "error", "message": str(exc)})


def compact_buffer(state):
    max_samples = state.sample_rate * MAX_BUFFER_SECONDS
    total = state.total_samples
    if total <= max_samples:
        return

    kept = []
    running = 0
    for chunk in reversed(state.chunks):
        kept.append(chunk)
        running += chunk.size
        if running >= max_samples:
            break
    state.chunks = list(reversed(kept))
    state.last_partial_samples = min(state.last_partial_samples, state.total_samples)


for line in sys.stdin:
    request = json.loads(line)
    request_type = request.get("type")

    if request_type == "start":
        log(f"start engine={request.get('engine', 'mlx_whisper')} model={MODEL} default_language={normalize_language(DEFAULT_LANGUAGE) or 'auto'} task={TASK} temperature={TEMPERATURE} best_of={BEST_OF} beam_size={BEAM_SIZE}")
        emit({"type": "ready", "engine": request.get("engine", "mlx_whisper")})
    elif request_type == "audio_chunk":
        channel_id = request.get("channelId", "default")
        frames = np.array(request.get("frames", []), dtype=np.float32)
        state = states[channel_id]
        state.sample_rate = int(request.get("sampleRate") or 16000)
        state.language = normalize_language(request.get("language") or state.language or "")
        state.prompt = str(request.get("prompt") or state.prompt or "").strip()
        if frames.size == 0:
            continue
        state.chunks.append(frames)
        compact_buffer(state)

        rms = float(np.sqrt(np.mean(np.square(frames)))) if frames.size > 0 else 0.0
        peak = float(np.max(np.abs(frames))) if frames.size > 0 else 0.0
        if rms < VAD_RMS_GATE and peak < VAD_PEAK_GATE:
            log(f"chunk skipped by VAD channel={channel_id} rms={rms:.6f} peak={peak:.6f}")
            continue

        partial_interval = max(1, int((state.sample_rate * PARTIAL_INTERVAL_MS) / 1000))
        if state.total_samples-state.last_partial_samples >= partial_interval:
            transcribe_channel(channel_id, final=False)
            state.last_partial_samples = state.total_samples

        if state.total_samples >= state.sample_rate * FINAL_WINDOW_SECONDS:
            transcribe_channel(channel_id, final=True)
            state.chunks.clear()
            state.last_partial_samples = 0
    elif request_type == "stop":
        for channel_id in list(states.keys()):
            transcribe_channel(channel_id, final=True)
        log(f"stop channels={len(states)}")
        emit({"type": "stopped", "engine": request.get("engine", "mlx_whisper")})
        break