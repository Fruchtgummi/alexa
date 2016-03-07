package alexa

import (
	"bytes"
	"encoding/binary"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"os/exec"
	"os/signal"

	"github.com/evanphx/alexa/config"
	"github.com/fatih/color"
	"github.com/gordonklaus/portaudio"
)

const DefaultQuietFrames = 30

func max(buf []int16) int16 {
	var max int16

	for _, s := range buf {
		if s > max {
			max = s
		}
	}

	return max
}

type AskCommand struct {
}

type State int

const (
	Waiting State = iota
	Listening
	Asking
)

func (r *AskCommand) Execute(args []string) error {
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

type ListenOpts struct {
	State       func(State)
	QuietFrames int
}

func Listen(opts ListenOpts) error {
	portaudio.Initialize()
	defer portaudio.Terminate()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)

	defer signal.Reset(os.Interrupt, os.Kill)

	in := make([]int16, 512)
	stream, err := portaudio.OpenDefaultStream(1, 0, 16000, len(in), in)
	if err != nil {
		return err
	}
	defer stream.Close()

	err = stream.Start()
	if err != nil {
		return err
	}

	var (
		buf            bytes.Buffer
		heardSomething bool
		quiets         int
		quietFrames    = opts.QuietFrames
	)

	if quietFrames == 0 {
		quietFrames = DefaultQuietFrames
	}

	if opts.State != nil {
		opts.State(Waiting)
	}

reader:
	for {
		err = stream.Read()
		if err != nil {
			return err
		}

		err = binary.Write(&buf, binary.LittleEndian, in)
		if err != nil {
			return err
		}

		if max(in) > 1000 {
			if heardSomething {
				if quiets > 0 {
					quiets /= 2
				}
			} else {
				heardSomething = true
				if opts.State != nil {
					opts.State(Listening)
				}
			}
		} else if heardSomething {
			quiets++

			if quiets == 30 {
				break reader
			}
		}

		select {
		case <-sig:
			break reader
		default:
		}
	}

	err = stream.Stop()
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
