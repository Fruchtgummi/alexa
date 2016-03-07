package alexa

import (
	"bytes"
	"os/exec"
)

func OSXMuted() (bool, error) {
	cmd := exec.Command("osascript", "-e", "output muted of (get volume settings)")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}

	return bytes.Equal(bytes.TrimSpace(output), []byte("true")), nil
}

func OSXMute() error {
	cmd := exec.Command("osascript", "-e", "set volume output muted true")
	return cmd.Run()
}

func OSXUnmute() error {
	cmd := exec.Command("osascript", "-e", "set volume output muted false")
	return cmd.Run()
}
