package conf

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

func (p *Conf) Watch(filePath, xDnsPath string, sDnsPath string, reload func()) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("new watcher error: %s", err)
	}
	go func() {
		var (
			timer      *time.Timer
			timerC     <-chan time.Time
			lastName   string
			lastChange time.Time
		)
		defer watcher.Close()
		for {
			select {
			case e := <-watcher.Events:
				if e.Has(fsnotify.Chmod) {
					continue
				}
				// Debounce: editors often emit multiple events in bursts.
				// Keep it single-goroutine and serial to avoid racey reload storms.
				lastName = e.Name
				lastChange = time.Now()
				if timer == nil {
					timer = time.NewTimer(1200 * time.Millisecond)
					timerC = timer.C
				} else {
					if !timer.Stop() {
						// Drain if needed
						select {
						case <-timer.C:
						default:
						}
					}
					timer.Reset(1200 * time.Millisecond)
					timerC = timer.C
				}
			case err := <-watcher.Errors:
				if err != nil {
					log.Printf("File watcher error: %s", err)
				}
			case <-timerC:
				// Perform reload once per debounce window
				name := filepath.Base(strings.TrimSuffix(lastName, "~"))
				switch name {
				case filepath.Base(xDnsPath), filepath.Base(sDnsPath):
					log.Println("DNS file changed, reloading...")
				default:
					log.Println("config file changed, reloading...")
				}

				// Reset config in-place then reload from disk.
				*p = *New()
				if err := p.LoadFromPath(filePath); err != nil {
					log.Printf("reload config error: %s", err)
				}
				reload()
				log.Printf("reload config success (lastChange=%s)", lastChange.Format(time.RFC3339))
			}
		}
	}()
	err = watcher.Add(filePath)
	if err != nil {
		return fmt.Errorf("watch file error: %s", err)
	}
	if xDnsPath != "" {
		err = watcher.Add(xDnsPath)
		if err != nil {
			return fmt.Errorf("watch dns file error: %s", err)
		}
	}
	if sDnsPath != "" {
		err = watcher.Add(sDnsPath)
		if err != nil {
			return fmt.Errorf("watch dns file error: %s", err)
		}
	}
	return nil
}
