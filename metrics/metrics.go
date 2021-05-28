package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"net/http"
	"roob.re/wallabot/database"
	"roob.re/wallabot/search"
	"roob.re/wallabot/telegram"
	"time"
)

const defaultInterval = 20 * time.Second

type Reporter struct {
	registry *prometheus.Registry

	Interval time.Duration
}

func New() *Reporter {
	return &Reporter{
		registry: prometheus.NewRegistry(),
		Interval: defaultInterval,
	}
}

func (r *Reporter) ListenAndServe(address string) error {
	promHandler := promhttp.HandlerFor(r.registry, promhttp.HandlerOpts{})
	return http.ListenAndServe(address, promHandler)
}

func (r *Reporter) Watch(db *database.Database, bot *telegram.Wallabot, se *search.Searcher) {
	r.watchDBMetrics(db)
	r.watchTelegramMetrics(bot)
	r.watchBacklogMetrics(se)
}

func (r *Reporter) watchDBMetrics(db *database.Database) {
	go func() {
		usersMetric := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "users",
		})
		_ = r.registry.Register(usersMetric)

		searchesMetric := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "searches",
		})
		_ = r.registry.Register(searchesMetric)

		notificationsMetric := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "notifications",
		})
		_ = r.registry.Register(notificationsMetric)

		for {
			users := 0
			searches := 0
			notifications := 0

			err := db.UserEach(func(u *database.User) error {
				users += 1
				searches += len(u.Searches)
				for _, s := range u.Searches {
					notifications += len(s.SentItems)
				}

				return nil
			})

			if err != nil {
				log.Warnf("Error while gathering metrics from database: %v", err)
			}

			usersMetric.Set(float64(users))
			searchesMetric.Set(float64(searches))
			notificationsMetric.Set(float64(notifications))

			time.Sleep(r.Interval)
		}
	}()
}

func (r *Reporter) watchTelegramMetrics(bot *telegram.Wallabot) {
	go func() {
		tgNotificationOffset := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "telegram_offset",
		})
		_ = r.registry.Register(tgNotificationOffset)

		for {
			tgNotificationOffset.Set(float64(len(bot.Notify)))

			time.Sleep(r.Interval / 4)
		}
	}()
}

func (r *Reporter) watchBacklogMetrics(searcher *search.Searcher) {
	go func() {
		searchBacklogOffset := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "searches_offset",
		})
		_ = r.registry.Register(searchBacklogOffset)

		searchBacklogCapacity := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "searches_capacity",
		})
		_ = r.registry.Register(searchBacklogOffset)

		for {
			l, c := searcher.BacklogStats()

			searchBacklogOffset.Set(float64(l))
			searchBacklogCapacity.Set(float64(c))

			time.Sleep(r.Interval / 4)
		}
	}()
}
