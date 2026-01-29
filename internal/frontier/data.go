package frontier

// Crawl state & ordering

type CrawlingPolicy struct {
	urlNode     URLNode
	depth       Depth
	queueEntity QueueEntity
}

type URLNode struct{}

type Depth struct{}

type QueueEntity struct{}
