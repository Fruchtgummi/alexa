package alexa

import (
	"fmt"

	"github.com/evanphx/alexa/portaudio"
)

type AudioCommand struct {
}

func (a *AudioCommand) Execute(args []string) error {
	err := portaudio.Initialize()
	if err != nil {
		return err
	}

	devices, err := portaudio.Devices()
	if err != nil {
		return err
	}

	for _, device := range devices {
		fmt.Printf("%s: input=%d output=%d\n", device.Name, device.MaxInputChannels, device.MaxOutputChannels)
	}

	return nil
}
