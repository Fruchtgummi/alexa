package alexa

import (
	"bytes"
	"encoding/binary"
	"os"
	"os/signal"
	"time"

	"github.com/Fruchtgummi/alexa/portaudio"
)

const DefaultQuietTime = time.Second

func ListenIntoBuffer(opts ListenOpts) (*bytes.Buffer, error) {
	portaudio.Initialize()
	defer portaudio.Terminate()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)

	defer signal.Reset(os.Interrupt, os.Kill)

	in := make([]int16, 8196)
	stream, err := portaudio.OpenDefaultStream(1, 0, 16000, len(in), in)
	if err != nil {
		return nil, err
	}

	defer stream.Close()

	err = stream.Start()
	if err != nil {
		return nil, err
	}

	var (
		buf            bytes.Buffer
		heardSomething bool
		quiet          bool
		quietTime      = opts.QuietDuration
		quietStart     time.Time
		lastFlux       float64
	)

	vad := NewVAD(len(in))

	if quietTime == 0 {
		quietTime = DefaultQuietTime
	}

	if opts.State != nil {
		opts.State(Waiting)
	}

reader:
	for {
		err = stream.Read()
		if err != nil {
			return nil, err
		}

		err = binary.Write(&buf, binary.LittleEndian, in)
		if err != nil {
			return nil, err
		}

		flux := vad.Flux(in)

		if lastFlux == 0 {
			lastFlux = flux
			continue
		}

		if heardSomething {
			if flux*1.75 <= lastFlux {
				if !quiet {
					quietStart = time.Now()
				} else {
					diff := time.Since(quietStart)

					if diff > quietTime {
						break reader
					}
				}

				quiet = true
			} else {
				quiet = false
				lastFlux = flux
			}
		} else {
			if flux >= lastFlux*1.75 {
				heardSomething = true
				if opts.State != nil {
					opts.State(Listening)
				}
			}

			lastFlux = flux
		}

		select {
		case <-sig:
			break reader
		default:
		}
	}

	err = stream.Stop()
	if err != nil {
		return nil, err
	}

	return &buf, nil
}
