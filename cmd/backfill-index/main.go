package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/zhoujianlin/ShareO/internal/config"
	"github.com/zhoujianlin/ShareO/internal/pkg/queue"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

func main() {
	configPath := os.Getenv("SHAREO_CONFIG")
	if configPath == "" {
		configPath = "config.yaml"
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatal(err)
	}
	if err := repository.InitDB(cfg.Database, cfg.Server.Mode); err != nil {
		log.Fatal(err)
	}
	if err := repository.InitRedis(cfg.Redis); err != nil {
		log.Fatal(err)
	}
	q := queue.New(repository.RDB)
	repo := repository.NewPostRepo()
	ctx := context.Background()
	const pageSize = 100
	page := 1
	count := 0
	for {
		posts, total, err := repo.Feed(repository.FeedQuery{Status: "approved", Page: page, PageSize: pageSize})
		if err != nil {
			log.Fatal(err)
		}
		for _, post := range posts {
			if err := q.PublishIndexPost(ctx, "upsert", post.ID); err != nil {
				log.Fatal(err)
			}
			count++
		}
		if len(posts) == 0 || int64(page*pageSize) >= total {
			break
		}
		page++
	}
	fmt.Printf("queued %d approved posts for image indexing\n", count)
}
