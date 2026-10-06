// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/aws/aws-sdk-go/service/ses/sesiface"
)

const (
	// CloudWatch Logs limits: 256 KB per event (26 bytes overhead), 1 MB and 10,000 events per batch.
	cwMaxEventBytes = 256*1024 - 26
	cwMaxBatchBytes = 1024*1024 - 26*10000
	cwMaxBatchCount = 10000
	cwEventOverhead = 26
)

// LogShipper copies the run log into a CloudWatch Logs stream (group and stream are created on demand).
type LogShipper struct {
	Client cloudwatchlogsiface.CloudWatchLogsAPI
	Group  string
	Stream string
	Now    func() time.Time
}

// Ship writes data line by line as log events and returns the number of events written.
func (s *LogShipper) Ship(ctx context.Context, data []byte) (int, error) {
	if s.Client == nil {
		return 0, ErrNotConfigured
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	if _, err := s.Client.CreateLogGroupWithContext(ctx, &cloudwatchlogs.CreateLogGroupInput{LogGroupName: aws.String(s.Group)}); err != nil && !isAlreadyExists(err) {
		return 0, fmt.Errorf("create log group %s: %w", s.Group, err)
	}
	if _, err := s.Client.CreateLogStreamWithContext(ctx, &cloudwatchlogs.CreateLogStreamInput{LogGroupName: aws.String(s.Group), LogStreamName: aws.String(s.Stream)}); err != nil && !isAlreadyExists(err) {
		return 0, fmt.Errorf("create log stream %s: %w", s.Stream, err)
	}
	ts := now().UnixMilli()
	var events []*cloudwatchlogs.InputLogEvent
	for _, line := range splitEvents(data) {
		events = append(events, &cloudwatchlogs.InputLogEvent{Message: aws.String(line), Timestamp: aws.Int64(ts)})
	}
	var token *string
	sent := 0
	for len(events) > 0 {
		n, size := 0, 0
		for n < len(events) && n < cwMaxBatchCount {
			size += len(*events[n].Message) + cwEventOverhead
			if size > cwMaxBatchBytes && n > 0 {
				break
			}
			n++
		}
		out, err := s.Client.PutLogEventsWithContext(ctx, &cloudwatchlogs.PutLogEventsInput{
			LogGroupName: aws.String(s.Group), LogStreamName: aws.String(s.Stream), LogEvents: events[:n], SequenceToken: token,
		})
		if err != nil {
			return sent, fmt.Errorf("put log events: %w", err)
		}
		if out != nil {
			token = out.NextSequenceToken
		}
		sent += n
		events = events[n:]
	}
	return sent, nil
}

// splitEvents splits the log into lines, chunking lines longer than the event limit; empty lines are kept
// as a single space so line numbering matches run.log.
func splitEvents(data []byte) []string {
	var out []string
	for _, line := range bytes.Split(bytes.TrimRight(data, "\n"), []byte("\n")) {
		if len(line) == 0 {
			out = append(out, " ")
			continue
		}
		for len(line) > cwMaxEventBytes {
			out = append(out, string(line[:cwMaxEventBytes]))
			line = line[cwMaxEventBytes:]
		}
		out = append(out, string(line))
	}
	return out
}

func isAlreadyExists(err error) bool {
	var aerr awserr.Error
	return errors.As(err, &aerr) && aerr.Code() == cloudwatchlogs.ErrCodeResourceAlreadyExistsException
}

// Mailer sends the report through SES SendRawEmail.
type Mailer struct {
	Client sesiface.SESAPI
}

// Send delivers a raw MIME message and returns the SES message id.
func (m Mailer) Send(ctx context.Context, from string, to []string, raw []byte) (string, error) {
	if m.Client == nil {
		return "", ErrNotConfigured
	}
	if len(to) == 0 {
		return "", errors.New("no recipients")
	}
	if len(raw) > MaxRawEmailBytes {
		return "", fmt.Errorf("message is %d bytes, SES limit is %d", len(raw), MaxRawEmailBytes)
	}
	out, err := m.Client.SendRawEmailWithContext(ctx, &ses.SendRawEmailInput{
		Source:       aws.String(from),
		Destinations: aws.StringSlice(to),
		RawMessage:   &ses.RawMessage{Data: raw},
	})
	if err != nil {
		return "", err
	}
	return aws.StringValue(out.MessageId), nil
}
