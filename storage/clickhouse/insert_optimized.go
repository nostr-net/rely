package clickhouse

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

// batchInsertOptimized uses standard SQL batch for better compatibility
func (s *Storage) batchInsertOptimized(ctx context.Context, events []*nostr.Event) error {
	if len(events) == 0 {
		return nil
	}

	// Use standard SQL prepared statement (compatible with database/sql)
	stmt, err := s.db.PrepareContext(ctx, `
		INSERT INTO nostr.events (
			id, pubkey, created_at, kind, content, sig,
			tags, tag_e, tag_p, tag_a, tag_t, tag_d, tag_g, tag_r,
			relay_received_at, version
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	now := uint32(time.Now().Unix())

	// Process all events
	for _, event := range events {
		// Single-pass tag extraction (CRITICAL OPTIMIZATION)
		extracted := extractAllTags(event.Tags)

		// Execute prepared statement
		_, err := stmt.ExecContext(ctx,
			event.ID,
			event.PubKey,
			uint32(event.CreatedAt),
			uint16(event.Kind),
			event.Content,
			event.Sig,
			extracted.tagsArray,
			extracted.e,
			extracted.p,
			extracted.a,
			extracted.t,
			extracted.d,
			extracted.g,
			extracted.r,
			now,  // relay_received_at
			now,  // version
		)
		if err != nil {
			return fmt.Errorf("failed to execute statement for event %s: %w", event.ID, err)
		}
	}

	return nil
}

// batchInserterOptimized is the optimized version of the batch inserter goroutine
func (s *Storage) batchInserterOptimized() {
	defer close(s.batchDone)

	// Pre-allocate buffer to avoid reallocations
	buffer := make([]*nostr.Event, 0, s.batchSize)
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(buffer) == 0 {
			return
		}

		start := time.Now()

		// Use standard batch insert for compatibility
		err := s.batchInsert(context.Background(), buffer)

		if err != nil {
			log.Printf("batch insert error: %v", err)
		} else {
			duration := time.Since(start)
			rate := float64(len(buffer)) / duration.Seconds()
			log.Printf("inserted batch of %d events in %s (%.0f events/sec)",
				len(buffer), duration, rate)
		}

		// Reuse buffer (avoid reallocation)
		buffer = buffer[:0]
	}

	for {
		select {
		case <-s.stopBatch:
			flush()
			return

		case <-ticker.C:
			flush()

		case event, ok := <-s.batchChan:
			if !ok {
				flush()
				return
			}

			buffer = append(buffer, event)
			if len(buffer) >= s.batchSize {
				flush()
			}
		}
	}
}

