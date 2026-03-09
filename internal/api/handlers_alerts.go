package api

import (
	"net/http"
	"time"
)

func (s *Server) handleAlerts(w http.ResponseWriter, _ *http.Request) {
	if s.alertEngine == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"alerts": []interface{}{},
			"total":  0,
		})
		return
	}

	events := s.alertEngine.RecentEvents(0) // 0 = all
	items := make([]map[string]interface{}, 0, len(events))
	for _, e := range events {
		item := map[string]interface{}{
			"rule":      e.Rule,
			"severity":  e.Severity,
			"message":   e.Message,
			"timestamp": e.Timestamp.UTC().Format(time.RFC3339),
		}
		if e.Source != "" {
			item["source"] = e.Source
		}
		if e.Count > 0 {
			item["count"] = e.Count
		}
		items = append(items, item)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"alerts": items,
		"total":  len(items),
	})
}

func (s *Server) handleAlertRules(w http.ResponseWriter, _ *http.Request) {
	if s.alertEngine == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"rules": []interface{}{},
		})
		return
	}

	rules := s.alertEngine.Rules()
	stats := s.alertEngine.RuleStats()

	items := make([]map[string]interface{}, 0, len(rules))
	for _, r := range rules {
		item := map[string]interface{}{
			"name":     r.Name,
			"type":     string(r.Type),
			"severity": string(r.Severity),
			"enabled":  true,
		}
		if st, ok := stats[r.Name]; ok {
			item["fire_count"] = st.FireCount
			if !st.LastFired.IsZero() {
				item["last_fired"] = st.LastFired.UTC().Format(time.RFC3339)
			}
		}
		items = append(items, item)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"rules": items,
	})
}
