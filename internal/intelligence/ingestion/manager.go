package ingestion

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/ignacio/lumo/internal/intelligence/vectorstore"
)

type Manager struct {
	store    vectorstore.VectorStore
	builder  *DocumentBuilder
	cfg      *config.RAGConfig
	log      *logrus.Logger

	// Batch queue
	queue    chan *vectorstore.Document
	stopChan chan struct{}
}

func NewManager(store vectorstore.VectorStore, cfg *config.RAGConfig, log *logrus.Logger) *Manager {
	return &Manager{
		store:    store,
		builder:  NewDocumentBuilder(),
		cfg:      cfg,
		log:      log,
		queue:    make(chan *vectorstore.Document, 100),
		stopChan: make(chan struct{}),
	}
}

// Start background ingestion worker
func (m *Manager) Start(ctx context.Context) {
	switch m.cfg.IngestionMode {
	case "realtime":
		go m.realtimeWorker(ctx)
	case "batch":
		go m.batchWorker(ctx)
	case "hybrid":
		go m.hybridWorker(ctx)
	default:
		m.log.Warn("Unknown ingestion mode, using hybrid")
		go m.hybridWorker(ctx)
	}
}

func (m *Manager) Stop() {
	close(m.stopChan)
}

// IngestReport adds a diagnostic report to the RAG system
func (m *Manager) IngestReport(ctx context.Context, report *diagnostics.Report, hostname string) error {
	doc, err := m.builder.BuildFromReport(report, hostname)
	if err != nil {
		return err
	}

	// Queue for background processing
	select {
	case m.queue <- doc:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Queue full, process synchronously
		m.log.Warn("Ingestion queue full, processing synchronously")
		return m.store.Store(ctx, doc)
	}
}

// IngestRemediation adds a remediation outcome to RAG
func (m *Manager) IngestRemediation(ctx context.Context, actionType string, outcome string, details string) error {
	doc, err := m.builder.BuildFromRemediation(actionType, outcome, details)
	if err != nil {
		return err
	}

	select {
	case m.queue <- doc:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return m.store.Store(ctx, doc)
	}
}

// Realtime worker: processes documents immediately
func (m *Manager) realtimeWorker(ctx context.Context) {
	m.log.Info("Starting realtime ingestion worker")

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case doc := <-m.queue:
			if err := m.store.Store(ctx, doc); err != nil {
				m.log.WithError(err).Error("Failed to store document")
			} else {
				m.log.WithField("doc_id", doc.ID).Debug("Document ingested")
			}
		}
	}
}

// Batch worker: accumulates documents and stores in batches
func (m *Manager) batchWorker(ctx context.Context) {
	m.log.Info("Starting batch ingestion worker")

	ticker := time.NewTicker(time.Duration(m.cfg.BatchInterval) * time.Second)
	defer ticker.Stop()

	batch := make([]*vectorstore.Document, 0, 100)

	flush := func() {
		if len(batch) == 0 {
			return
		}

		if err := m.store.StoreBatch(ctx, batch); err != nil {
			m.log.WithError(err).Error("Failed to store batch")
		} else {
			m.log.WithField("count", len(batch)).Info("Batch ingested")
		}

		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case <-m.stopChan:
			flush()
			return
		case <-ticker.C:
			flush()
		case doc := <-m.queue:
			batch = append(batch, doc)
			if len(batch) >= 100 {
				flush()
			}
		}
	}
}

// Hybrid worker: realtime for high-severity, batch for low-severity
func (m *Manager) hybridWorker(ctx context.Context) {
	m.log.Info("Starting hybrid ingestion worker")

	ticker := time.NewTicker(time.Duration(m.cfg.BatchInterval) * time.Second)
	defer ticker.Stop()

	batch := make([]*vectorstore.Document, 0, 100)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := m.store.StoreBatch(ctx, batch); err != nil {
			m.log.WithError(err).Error("Failed to store batch")
		} else {
			m.log.WithField("count", len(batch)).Info("Batch ingested")
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case <-m.stopChan:
			flush()
			return
		case <-ticker.C:
			flush()
		case doc := <-m.queue:
			// Check severity
			if severity, ok := doc.Metadata[vectorstore.MetadataSeverity].(string); ok {
				if severity == "CRITICAL" || severity == "HIGH" || severity == "ERROR" {
					// Process immediately
					if err := m.store.Store(ctx, doc); err != nil {
						m.log.WithError(err).Error("Failed to store critical document")
					} else {
						m.log.WithField("doc_id", doc.ID).Debug("Critical document ingested immediately")
					}
					continue
				}
			}

			// Batch low-severity
			batch = append(batch, doc)
			if len(batch) >= 100 {
				flush()
			}
		}
	}
}
