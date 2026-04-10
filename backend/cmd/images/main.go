package main

import (
	"context"
	"openreplay/backend/pkg/db/postgres/pool"
	"openreplay/backend/pkg/db/redis"
	"openreplay/backend/pkg/images/service"
	"openreplay/backend/pkg/metrics/database"
	"openreplay/backend/pkg/metrics/web"
	"openreplay/backend/pkg/server"
	"openreplay/backend/pkg/server/api"
	"openreplay/backend/pkg/server/middleware"
	"os"
	"os/signal"
	"syscall"
	"time"

	config "openreplay/backend/internal/config/images"
	"openreplay/backend/pkg/health"
	imageService "openreplay/backend/pkg/images"
	"openreplay/backend/pkg/logger"
	"openreplay/backend/pkg/messages"
	"openreplay/backend/pkg/metrics"
	imagesMetrics "openreplay/backend/pkg/metrics/images"
	"openreplay/backend/pkg/objectstorage/store"
	"openreplay/backend/pkg/queue"
	"openreplay/backend/pkg/queue/types"
)

func main() {
	ctx := context.Background()
	log := logger.New()
	cfg := config.New(log)

	h := health.New()

	imageMetrics := imagesMetrics.New("images")
	webMetrics := web.New("images")
	dbMetric := database.New("images")
	metrics.New(log, append(imageMetrics.List(), append(webMetrics.List(), dbMetric.List()...)...))

	pgConn, err := pool.New(dbMetric, cfg.Postgres.String())
	if err != nil {
		log.Fatal(ctx, "can't init postgres connection: %s", err)
	}
	defer pgConn.Close()

	redisConn, err := redis.New(&cfg.Redis)
	if err != nil {
		log.Warn(ctx, "can't init redis connection: %s", err)
	}
	defer redisConn.Close()

	objStore, err := store.NewStore(&cfg.ObjectsConfig)
	if err != nil {
		log.Fatal(ctx, "can't init object storage: %s", err)
	}

	producer := queue.NewProducer(cfg.MessageSizeLimit, true)
	defer producer.Close(15000)
	h.Register("producer", func(ctx context.Context) error {
		return producer.Ping(ctx)
	})

	srv, err := service.New(cfg, log, objStore, imageMetrics)
	if err != nil {
		log.Fatal(ctx, "can't init images service: %s", err)
	}

	consumer, err := queue.NewConsumer(
		log,
		cfg.GroupImageStorage,
		[]string{
			cfg.TopicRawImages,
		},
		messages.NewImagesMessageIterator(srv.MessageIterator, nil, true),
		false,
		cfg.MessageSizeLimit,
		nil,
		types.NoReadBackGap,
	)
	if err != nil {
		log.Fatal(ctx, "can't init message consumer: %s", err)
	}
	h.Register("consumer", func(ctx context.Context) error {
		return consumer.Ping(ctx)
	})

	services, err := imageService.NewServiceBuilder(log, cfg, webMetrics, dbMetric, producer, pgConn, redisConn)

	middlewares, err := middleware.NewMinimalMiddlewareBuilder(&cfg.HTTP)
	if err != nil {
		log.Fatal(ctx, "failed while creating minimal http middleware: %s", err)
	}

	router, err := api.NewRouter(log, &cfg.HTTP, api.NoPrefix, services.Handlers(), middlewares.Middlewares())
	if err != nil {
		log.Fatal(ctx, "failed while creating router: %s", err)
	}

	go server.Run(ctx, log, &cfg.HTTP, router)

	log.Info(ctx, "Images service started")

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	counterTick := time.Tick(time.Second * 30)
	for {
		select {
		case sig := <-sigchan:
			log.Info(ctx, "Caught signal %v: terminating", sig)
			srv.Wait()
			consumer.Close()
			os.Exit(0)
		case <-counterTick:
			srv.Wait()
			if err := consumer.Commit(); err != nil {
				log.Error(ctx, "can't commit messages: %s", err)
			}
		default:
			err := consumer.ConsumeNext()
			if err != nil {
				log.Fatal(ctx, "Error on images consumption: %v", err)
			}
		}
	}
}
