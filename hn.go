package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

// ── HN API ────────────────────────────────────────────────────────────────────

const baseURL = "https://hacker-news.firebaseio.com/v0"

// Item represents a Hacker News story.
type Item struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Score       int    `json:"score"`
	Descendants int    `json:"descendants"`
	Time        int64  `json:"time"`
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

func fetchJSON(url string, v any) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(v)
}

func fetchItem(id int) *Item {
	var item Item
	if err := fetchJSON(fmt.Sprintf("%s/item/%d.json", baseURL, id), &item); err != nil {
		return nil
	}
	if item.Title == "" {
		return nil
	}
	return &item
}

// fetchItemsParallel fetches items in parallel (throttled by a semaphore),
// and returns them sorted by submission time ascending.
func fetchItemsParallel(ids []int, throttle int) []*Item {
	if len(ids) == 0 {
		return nil
	}
	results := make([]*Item, len(ids))
	sem := make(chan struct{}, throttle)
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		i, id := i, id
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = fetchItem(id)
		}()
	}
	wg.Wait()

	var items []*Item
	for _, item := range results {
		if item != nil {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(a, b int) bool { return items[a].Time < items[b].Time })
	return items
}

// fetchFrontPage fetches the top 30 HN stories sorted by rank, and returns
// both the item slice and an id→rank (1-based) map.
func fetchFrontPage(throttle int) ([]*Item, map[int]int, error) {
	var ids []int
	if err := fetchJSON(baseURL+"/topstories.json", &ids); err != nil {
		return nil, nil, err
	}
	if len(ids) > 30 {
		ids = ids[:30]
	}
	rankMap := make(map[int]int, len(ids))
	for i, id := range ids {
		rankMap[id] = i + 1
	}
	items := fetchItemsParallel(ids, throttle)
	sort.Slice(items, func(a, b int) bool {
		return rankMap[items[a].ID] < rankMap[items[b].ID]
	})
	return items, rankMap, nil
}

func fetchNewStoryIDs() ([]int, error) {
	var ids []int
	return ids, fetchJSON(baseURL+"/newstories.json", &ids)
}
