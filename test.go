package alexa

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/evanphx/alexa/pocketsphinx"
	"github.com/fatih/color"
)

type TestCommand struct {
	Path string `short:"d" description:"directory with the sphinx stuff"`
}

func (t *TestCommand) Execute(args []string) error {
	c := color.New(color.Bold)

	var opts ListenOpts

	muted, err := OSXMuted()
	if err == nil && !muted {
		OSXMute()
		defer OSXUnmute()
	}

	opts.State = func(s State) {
		switch s {
		case Waiting:
			c.Println("Waiting...")
		case Listening:
			c.Println("Listening...")
		case Asking:
			OSXUnmute()
			c.Println("Asking...")
		}
	}

	return TestListen(t.Path, opts)
}

func TestListen(path string, opts ListenOpts) error {
	/*
		conf := pocketsphinx.Config{
			Hmm:       filepath.Join(path, "models/en-us/en-us"),
			Dict:      filepath.Join(path, "models/en-us/cmudict-en-us.dict"),
			Lm:        filepath.Join(path, "models/en-us/en-us.lm.bin"),
			Keyphrase: "hey alexa",
		}
	*/

	conf := pocketsphinx.Config{
		Hmm:  filepath.Join(path, "models/en-us/en-us"),
		Dict: filepath.Join(path, "models/alexa/9671.dic"),
		Lm:   filepath.Join(path, "models/alexa/9671.lm"),
	}

	buf, err := ListenIntoBuffer(opts)
	if err != nil {
		return err
	}

	ps := pocketsphinx.NewPocketSphinx(conf)
	defer ps.Free()

	results, err := ps.ProcessUtt(buf.Bytes(), 0, false)
	if err != nil {
		log.Printf("err: %s\n", err)
	}

	for _, res := range results {
		fmt.Printf("got: %s => %#v\n", res.Text, res)
	}
	return nil
}
