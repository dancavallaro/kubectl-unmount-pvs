package spinner

import (
	"time"

	"github.com/briandowns/spinner"
)

func Wait(label string, until func() (bool, error), onErr func(error), interval time.Duration) <-chan struct{} {
	ch := make(chan struct{})

	go func() {
		s := spinner.New(spinner.CharSets[70], 100*time.Millisecond)
		s.Prefix = label
		s.Start()

		for {
			done, err := until()
			if err != nil {
				onErr(err)
			}
			if done {
				break
			}
			<-time.NewTimer(interval).C
		}

		s.Stop()
		close(ch)
	}()

	return ch
}
