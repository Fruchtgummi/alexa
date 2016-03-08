package alexa

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/evanphx/alexa/config"
	"github.com/evanphx/alexa/pocketsphinx"
	"github.com/fatih/color"
)

type AskCommand struct {
	Prompt bool   `short:"p" description:"listen for prompt, then ask"`
	Path   string `short:"d" description:"directory with the sphinx stuff"`
}

type State int

const (
	Waiting State = iota
	Listening
	Asking
)

func (r *AskCommand) Execute(args []string) error {
	InitAudio()
	defer FreeAudio()

	if r.Prompt {
		return r.prompted()
	}

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

	return Listen(opts)
}

func (r *AskCommand) prompted() error {
	c := color.New(color.Bold)

	conf := pocketsphinx.Config{
		Hmm:  filepath.Join(r.Path, "models/en-us/en-us"),
		Dict: filepath.Join(r.Path, "models/alexa/9671.dic"),
		Lm:   filepath.Join(r.Path, "models/alexa/9671.lm"),
	}

	ps := pocketsphinx.NewPocketSphinx(conf)
	defer ps.Free()

	for {
		var opts ListenOpts

		opts.State = func(s State) {
			switch s {
			case Waiting:
				c.Println("Waiting for prompt...")
			case Listening:
				c.Println("Listening to see if this is the prompt...")
			case Asking:
				c.Println("Prompt heard!")
			}
		}

		buf, err := ListenIntoBuffer(opts)
		if err != nil {
			return err
		}

		results, err := ps.ProcessUtt(buf.Bytes(), 0, false)
		if err != nil {
			return err
		}

		if len(results) < 1 {
			c.Println("Unknown, retrying")
			continue
		}

		if results[0].Text != "HEY ALEXA" {
			c.Println("Unknown, retrying")
			continue
		}

		exec.Command("afplay", "beep.wav").Run()

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

		opts.AlreadyListening = true

		err = Listen(opts)
		if err != nil {
			return err
		}
	}
}

type ListenOpts struct {
	State            func(State)
	QuietDuration    time.Duration
	AlreadyListening bool
}

func Listen(opts ListenOpts) error {
	buf, err := ListenIntoBuffer(opts)
	if err != nil {
		return err
	}

	if opts.State != nil {
		opts.State(Asking)
	}

	url := "https://access-alexa-na.amazon.com/v1/avs/speechrecognizer/recognize"
	d := `{
		"messageHeader": {
			"deviceContext": [
			{
				"name": "playbackState",
				"namespace": "AudioPlayer",
				"payload": {
					"streamId": "",
					"offsetInMilliseconds": "0",
					"playerActivity": "IDLE"
				}
			}
			]
		},
		"messageBody": {
			"profile": "alexa-close-talk",
			"locale": "en-us",
			"format": "audio/L16; rate=16000; channels=1"
		}
	}`

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="request"`)
	h.Set("Content-Type", "application/json; charset=UTF-8")

	part, err := writer.CreatePart(h)
	if err != nil {
		return err
	}

	part.Write([]byte(d))

	h = make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="audio"`)
	h.Set("Content-Type", "audio/L16; rate=16000; channels=1")

	part, err = writer.CreatePart(h)
	if err != nil {
		return err
	}

	part.Write(buf.Bytes())

	err = writer.Close()
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}

	token, err := config.GetToken()
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+writer.Boundary())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return err
	}

	mr := multipart.NewReader(resp.Body, params["boundary"])
	for {
		p, err := mr.NextPart()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		if p.Header.Get("Content-Type") == "audio/mpeg" {
			cmd := exec.Command("mpg123", "-q", "-")
			ip, err := cmd.StdinPipe()
			if err != nil {
				return err
			}

			err = cmd.Start()
			if err != nil {
				return err
			}

			io.Copy(ip, p)

			ip.Close()

			return cmd.Wait()
		}
	}

	return nil
}
