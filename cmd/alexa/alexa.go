package main

import (
	"github.com/evanphx/alexa"
	"github.com/jessevdk/go-flags"
)

func main() {
	parser := flags.NewParser(&alexa.Globals, flags.Default)

	parser.AddCommand("audio", "list audio devices", "", &alexa.AudioCommand{})
	parser.AddCommand("setup", "start the setup procedure", "", &alexa.SetupCommand{})
	parser.AddCommand("ask", "send alexa a question", "", &alexa.AskCommand{})

	parser.Parse()
}
