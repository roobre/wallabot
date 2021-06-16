package search

import (
	log "github.com/sirupsen/logrus"
	"math/rand"
	"roob.re/wallabot/database"
	"roob.re/wallabot/wallapop"
	"time"
)

const batchSearchInterval = 30 * time.Minute
const workers = 2

type Searcher struct {
	db       *database.Database
	wp       *wallapop.Client
	notifier chan<- database.Notification
	backlog  chan job
}

type job struct {
	user  *database.User
	seach *database.SavedSearch
}

func New(db *database.Database, wp *wallapop.Client, notifier chan<- database.Notification) *Searcher {
	return &Searcher{
		db:       db,
		wp:       wp,
		notifier: notifier,
		backlog:  make(chan job, 128),
	}
}

func (s *Searcher) Start() {
	go s.fillBacklog()
	for i := 0; i < workers; i++ {
		go s.consumeBacklog()
	}
}

func (s *Searcher) fillBacklog() {
	lastFill := time.Now()

	for {
		time.Sleep(batchSearchInterval - time.Since(lastFill))

		jobs := make([]job, 0, 64)

		log.WithFields(log.Fields{
			"component": "search",
		}).Infof("Starting to gather search jobs...")

		_ = s.db.UserEach(func(u *database.User) error {
			for _, ss := range u.Searches {
				jobs = append(jobs, job{
					user:  u,
					seach: ss,
				})
			}

			return nil
		})

		log.WithFields(log.Fields{
			"component": "search",
		}).Infof("Gathered %d backlog jobs, queuing...\n", len(jobs))

		rand.Shuffle(len(jobs), func(i, j int) { jobs[i], jobs[j] = jobs[j], jobs[i] })
		for _, job := range jobs {
			s.backlog <- job
		}

		log.WithFields(log.Fields{
			"component": "search",
		}).Infof("%d jobs queued\n", len(jobs))

		lastFill = time.Now()
	}
}

func (s *Searcher) consumeBacklog() {
	for job := range s.backlog {
		lat, long := job.user.Location()
		items, err := s.wp.Search(wallapop.SearchArgs{
			Keywords: job.seach.Keywords,
			Latitude: lat,
			Longitude: long,
		})
		if err != nil {
			log.WithFields(log.Fields{
				"component": "search",
			}).Errorf("Error processing backlog search '%s' for user %d: %v", job.seach.Keywords, job.user.ID, err)
			continue
		}

		for i := range items {
			item := &items[i]
			if item.Price > job.seach.MaxPrice {
				continue
			}

			log.WithFields(log.Fields{
				"component": "search",
			}).Debugf("Found '%s' for '%s', queuing notification", item.ID, job.seach.Keywords)

			s.notifier <- database.Notification{
				User:   job.user,
				Item:   item,
				Search: job.seach.Keywords,
			}
		}

		time.Sleep(time.Duration(10000+rand.Intn(5000)) * time.Millisecond)
	}
}

func (s *Searcher) BacklogStats() (int, int) {
	return len(s.backlog), cap(s.backlog)
}
