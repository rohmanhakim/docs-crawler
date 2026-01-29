package robots

/*
Responsibilities

- Fetch robots.txt per host
- Cache rules for crawl duration
- Enforce allow/disallow rules before enqueue

Robots checks occur before a URL enters the frontier.
*/

type Robot struct {
}
