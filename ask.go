package alexa

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os/exec"
	"sort"
	"time"

	"github.com/evanphx/alexa/config"
	"github.com/fatih/color"
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

func avg(buf []int16) int16 {
	var tot int64

	for _, s := range buf {
		if s < 0 {
			s = -s
		}
		tot += int64(s)
	}

	return int16(tot / int64(len(buf)))
}

const silenceFloor = 327 // int16(0.01 * float64(math.MaxInt16))

func silent(buf []int16) float32 {
	var silent int

	for _, s := range buf {
		if s < silenceFloor && s > -silenceFloor {
			silent++
		}
	}

	return 100 * (float32(silent) / float32(len(buf)))
}

func variance(buf []int16) int16 {
	a := avg(buf)

	var m int16

	for _, s := range buf {
		diff := s - a

		if diff < 0 {
			diff = -diff
		}

		if diff > m {
			m = diff
		}
	}

	return m
}

type int16slice []int16

func (s int16slice) Len() int           { return len(s) }
func (s int16slice) Less(i, j int) bool { return s[i] < s[j] }
func (s int16slice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func nine95(buf, tmp []int16) int16 {
	copy(tmp, buf)

	spec := int16slice(tmp)
	sort.Sort(spec)

	pos := len(buf) - int(float32(len(buf))*0.05)

	return tmp[pos]
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
	State         func(State)
	QuietDuration time.Duration
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
