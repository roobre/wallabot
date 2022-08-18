package search

import (
	"math/rand"
	"time"

	log "github.com/sirupsen/logrus"
	"roob.re/wallabot/database"
	"roob.re/wallabot/wallapop"
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
	user        *database.User
	savedSearch *database.SavedSearch
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
	var lastFill time.Time

	for {
		time.Sleep(batchSearchInterval - time.Since(lastFill))

		jobs := make([]job, 0, 64)

		log.WithFields(log.Fields{
			"component": "search",
		}).Infof("Starting to gather search jobs...")

		_ = s.db.UserEach(func(u *database.User) error {
			for _, ss := range u.Searches {
				ss.LegacyFill()
				jobs = append(jobs, job{
					user:        u,
					savedSearch: ss,
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
		// Get search radius, and user radius as a fallback
		if job.savedSearch.Search.RadiusKm == 0 {
			job.savedSearch.Search.RadiusKm = job.user.RadiusKm
		}

		lat, long := job.user.Location()
		args := job.savedSearch.Search.Args()
		args.Latitude = lat
		args.Longitude = long

		items, err := s.wp.Search(args)
		if err != nil {
			log.WithFields(log.Fields{
				"component": "search",
			}).Errorf("Error processing backlog search %q for user %d: %v", job.savedSearch.Search.Keywords, job.user.ID, err)
			continue
		}

		for i := range items {
			item := &items[i]
			if int(item.Price) > job.savedSearch.Search.MaxPrice {
				continue
			}

			log.WithFields(log.Fields{
				"component": "search",
			}).Debugf("Found '%s' for %q, queuing notification", item.ID, job.savedSearch.Search.Keywords)

			s.notifier <- database.Notification{
				User:   job.user,
				Item:   item,
				Search: job.savedSearch.Search.Keywords,
			}
		}

		time.Sleep(time.Duration(10000+rand.Intn(5000)) * time.Millisecond)
	}
}

func (s *Searcher) BacklogStats() (int, int) {
	return len(s.backlog), cap(s.backlog)
}
