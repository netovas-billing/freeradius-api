package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"freeradius-api/database"
	"freeradius-api/models"
)

const httpTimeout = 10 * time.Second
const batchSize = 100

type Poller struct {
	interval time.Duration
	stop     chan struct{}
	wg       sync.WaitGroup
	client   *http.Client
}

func NewPoller(intervalSeconds int) *Poller {
	if intervalSeconds < 1 {
		intervalSeconds = 5
	}
	return &Poller{
		interval: time.Duration(intervalSeconds) * time.Second,
		stop:     make(chan struct{}),
		client:   &http.Client{Timeout: httpTimeout},
	}
}

func (p *Poller) Start() {
	p.wg.Add(1)
	go p.loop()
	log.Printf("webhook poller started (interval=%s)", p.interval)
}

func (p *Poller) Stop(ctx context.Context) {
	close(p.stop)
	done := make(chan struct{})
	go func() { p.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

func (p *Poller) loop() {
	defer p.wg.Done()
	t := time.NewTicker(p.interval)
	defer t.Stop()
	for {
		select {
		case <-p.stop:
			return
		case <-t.C:
			p.tick()
		}
	}
}

func (p *Poller) tick() {
	var hooks []models.Webhook
	if err := database.DB.Where("enabled = ?", true).Find(&hooks).Error; err != nil {
		log.Printf("webhook poller: list webhooks failed: %v", err)
		return
	}
	for i := range hooks {
		p.process(&hooks[i])
	}
}

// event payload sent to webhook URL
type eventEnvelope struct {
	Event     string      `json:"event"`
	Timestamp time.Time   `json:"timestamp"`
	WebhookID uint        `json:"webhook_id"`
	Data      interface{} `json:"data"`
}

func (p *Poller) process(w *models.Webhook) {
	switch w.Event {
	case "session_stop":
		p.processSessionStop(w)
	case "session_start":
		p.processSessionStart(w)
	case "auth_accept":
		p.processPostAuth(w, "Access-Accept")
	case "auth_reject":
		p.processPostAuth(w, "Access-Reject")
	}
}

func (p *Poller) processSessionStart(w *models.Webhook) {
	var rows []models.Radacct
	database.DB.Where("radacctid > ?", w.Cursor).
		Order("radacctid ASC").Limit(batchSize).Find(&rows)
	var newCursor uint64 = w.Cursor
	for _, r := range rows {
		ok := p.fire(w, r)
		if ok && r.RadAcctID > newCursor {
			newCursor = r.RadAcctID
		}
		if !ok {
			break
		}
	}
	if newCursor != w.Cursor {
		database.DB.Model(w).Update("cursor", newCursor)
	}
}

func (p *Poller) processSessionStop(w *models.Webhook) {
	// cursor disimpan sebagai unix millis acctstoptime
	cutoff := time.UnixMilli(int64(w.Cursor))
	var rows []models.Radacct
	database.DB.Where("acctstoptime IS NOT NULL AND acctstoptime > ?", cutoff).
		Order("acctstoptime ASC").Limit(batchSize).Find(&rows)
	var newCursor uint64 = w.Cursor
	for _, r := range rows {
		ok := p.fire(w, r)
		if ok && r.AcctStopTime != nil {
			ms := uint64(r.AcctStopTime.UnixMilli())
			if ms > newCursor {
				newCursor = ms
			}
		}
		if !ok {
			break
		}
	}
	if newCursor != w.Cursor {
		database.DB.Model(w).Update("cursor", newCursor)
	}
}

func (p *Poller) processPostAuth(w *models.Webhook, reply string) {
	var rows []models.Radpostauth
	database.DB.Where("id > ? AND reply = ?", w.Cursor, reply).
		Order("id ASC").Limit(batchSize).Find(&rows)
	var newCursor uint64 = w.Cursor
	for _, r := range rows {
		ok := p.fire(w, r)
		if ok && uint64(r.ID) > newCursor {
			newCursor = uint64(r.ID)
		}
		if !ok {
			break
		}
	}
	if newCursor != w.Cursor {
		database.DB.Model(w).Update("cursor", newCursor)
	}
}

func (p *Poller) fire(w *models.Webhook, data interface{}) bool {
	envelope := eventEnvelope{
		Event:     w.Event,
		Timestamp: time.Now().UTC(),
		WebhookID: w.ID,
		Data:      data,
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		p.logDelivery(w, 0, "marshal: "+err.Error(), 0)
		return false
	}

	mac := hmac.New(sha256.New, []byte(w.Secret))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequest(http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		p.logDelivery(w, 0, err.Error(), 0)
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-FreeRADIUS-Signature", sig)
	req.Header.Set("X-FreeRADIUS-Event", w.Event)

	start := time.Now()
	resp, err := p.client.Do(req)
	dur := time.Since(start).Milliseconds()

	if err != nil {
		p.logDelivery(w, 0, err.Error(), int(dur))
		p.bumpFailure(w)
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	now := time.Now()
	database.DB.Model(w).Updates(map[string]interface{}{
		"last_fired_at": &now,
	})

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		p.logDelivery(w, resp.StatusCode, "", int(dur))
		return true
	}
	p.logDelivery(w, resp.StatusCode, "non-2xx response", int(dur))
	p.bumpFailure(w)
	return false
}

func (p *Poller) logDelivery(w *models.Webhook, status int, errMsg string, durMs int) {
	if len(errMsg) > 512 {
		errMsg = errMsg[:512]
	}
	database.DB.Create(&models.WebhookDelivery{
		WebhookID:  w.ID,
		Event:      w.Event,
		URL:        w.URL,
		StatusCode: status,
		Error:      errMsg,
		Timestamp:  time.Now(),
		DurationMs: durMs,
	})
}

func (p *Poller) bumpFailure(w *models.Webhook) {
	database.DB.Model(w).UpdateColumn("failure_count", w.FailureCount+1)
}
