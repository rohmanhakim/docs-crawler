package robots

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
)

/*
Responsibilities

- Fetch robots.txt per host
- Cache rules for crawl duration
- Enforce allow/disallow rules before enqueue

Robots checks occur before a URL enters the frontier.
TODO:
Split robots API into:
- Decision (admission)
- Error (infrastructure)
*/

type Robot struct {
	metadataSink metadata.MetadataSink
}

func NewRobot(metadataSink metadata.MetadataSink) Robot {
	return Robot{
		metadataSink: metadataSink,
	}
}

func (r *Robot) Decide(url url.URL) (Decision, error) {
	decision, err := decide()
	if err != nil {
		var robotsError *RobotsError
		errors.As(err, &robotsError)
		r.metadataSink.RecordError(
			time.Now(),
			"robots",
			"Robot.Decide",
			mapRobotsErrorToMetadataCause(robotsError),
			err.Error(),
			[]metadata.Attribute{
				metadata.NewAttr(metadata.AttrURL, fmt.Sprintf("%v", url)),
			},
		)
		return Decision{}, robotsError
	}
	return decision, nil
}

func decide() (Decision, error) {
	return Decision{}, nil
}
