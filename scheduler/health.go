package scheduler

import (
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/raintank/worldping-api/pkg/log"
)

func (s *Scheduler) CheckHealth() {
	tickers := time.NewTicker(time.Duration(30) * time.Second)

	laststate := 1
	var wg sync.WaitGroup
	for t := range tickers.C {
		hcheckrets := make(chan int, len(s.HealthHosts))
		for _, _h := range s.HealthHosts {
			log.Info("CheckHealth at: %s with host: %s", t.String(), _h)
			h, err := url.Parse(_h)
			if err != nil {
				log.Error(3, "Invalid host format %s with err %s", h, err.Error())
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				client := http.Client{Timeout: time.Duration(2) * time.Second}
				req, err := http.NewRequest("GET", h.String(), nil)
				if err != nil {
					log.Error(3, "Error when create health check for host: %s failed with err: %s", h.String(), err.Error())
					hcheckrets <- 1
					return
				}
				resp, err := client.Do(req)
				if err != nil {
					log.Error(3, "Unhealthy when checking host: %s failed with err: %s", h.String(), err.Error())
					hcheckrets <- 1
					return
				}
				hcheckrets <- 0
				return
			}()
		}
		wg.Wait()
		close(hcheckrets)

		//Checking result for health
		newstate := 0
		failedcheck := 0
		for hchk := range hcheckrets {
			failedcheck += hchk
		}
		if float64(failedcheck) > float64(len(s.HealthHosts))/2 {
			newstate = 1
			log.Error(3, "Host unhealthy with %d failedcheck / %d totalcheck", failedcheck, len(s.HealthHosts))
		} else {
			log.Info("Host healthy with %d failedcheck / %d totalcheck", failedcheck, len(s.HealthHosts))
		}

		if newstate != laststate {
			if newstate == 1 { //stop all check
				log.Info("Host unhealthy, stop all check")
				s.Lock()
				s.Healthy = false
				for _, chkInstance := range s.Check {
					_checkInstance := chkInstance
					go _checkInstance.Stop()
				}
				s.Unlock()
			} else { //restart check
				log.Info("Host healthy, start all check")
				s.Lock()
				s.Healthy = true
				for _, chkInstance := range s.Check {
					_checkInstance := chkInstance
					log.Debug("Start check: %s for %s", _checkInstance.Check.Type, _checkInstance.Check.Settings["hostname"])
					go _checkInstance.Run()
				}
				s.Unlock()
			}
		}
		laststate = newstate
	}
}
