package speech

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

//go:embed helpers/mlx_whisper_worker.py
var mlxWhisperWorkerSource string

var (
	workerScriptOnce sync.Once
	workerScriptPath string
	workerScriptErr  error
)

func materializeMLXWorkerScript() (string, error) {
	workerScriptOnce.Do(func() {
		baseDir, err := os.UserCacheDir()
		if err != nil {
			baseDir = os.TempDir()
		}
		helperDir := filepath.Join(baseDir, "procom", "helpers")
		if err := os.MkdirAll(helperDir, 0o755); err != nil {
			workerScriptErr = fmt.Errorf("create worker helper directory: %w", err)
			return
		}
		path := filepath.Join(helperDir, "mlx_whisper_worker.py")
		if err := os.WriteFile(path, []byte(mlxWhisperWorkerSource), 0o755); err != nil {
			workerScriptErr = fmt.Errorf("write worker helper source: %w", err)
			return
		}
		workerScriptPath = path
	})
	if workerScriptErr != nil {
		return "", workerScriptErr
	}
	return workerScriptPath, nil
}
