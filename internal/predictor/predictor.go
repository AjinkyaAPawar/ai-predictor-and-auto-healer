package predictor

import (
	"fmt"
	"math"
	"ai-predictor-healer/internal/collector"
)

type Predictor struct {
	podHistory  map[string][]collector.PodMetrics
	nodeHistory map[string][]collector.NodeMetrics
}

type PredictionResult struct {
	PodName         string
	PodNamespace    string
	Risk            string
	Issues          []string
	Action          string
	Confidence      int
	Score           float64
	TimeToFailure   string
	Trend           string
	MemoryLeakRate  float64
	CPUGrowthRate   float64
	PredictionHours int
	Model           string
	WindowPoints    int
	Reason          string
	Signals         []Signal
}

type Signal struct {
	Name     string
	Current  float64
	Baseline float64
	Delta    float64
	Score    float64
	Severity string
	Note     string
}

type TrendAnalysis struct {
	CPUTrend      string
	MemTrend      string
	CPUSlope      float64
	MemSlope      float64
	IsMemoryLeak  bool
	IsCPUGrowing  bool
	HoursToFailure float64
}

func New() *Predictor {
	return &Predictor{
		podHistory:  make(map[string][]collector.PodMetrics),
		nodeHistory: make(map[string][]collector.NodeMetrics),
	}
}

func (p *Predictor) UpdateHistory(metrics []collector.PodMetrics) {
	for _, metric := range metrics {
		key := fmt.Sprintf("%s/%s", metric.Namespace, metric.Name)
		
		if p.podHistory[key] == nil {
			p.podHistory[key] = make([]collector.PodMetrics, 0)
		}
		
		p.podHistory[key] = append(p.podHistory[key], metric)
		
		// Keep last 20 measurements (10 minutes of history at 30s intervals)
		if len(p.podHistory[key]) > 20 {
			p.podHistory[key] = p.podHistory[key][1:]
		}
	}
}

func (p *Predictor) PredictIssues(currentMetrics []collector.PodMetrics) []PredictionResult {
	var predictions []PredictionResult
	
	for _, metric := range currentMetrics {
		key := fmt.Sprintf("%s/%s", metric.Namespace, metric.Name)
		history := p.podHistory[key]
		
		result := p.analyzePodAdvanced(metric, history)
		
		// Report issues with score > 30 OR predictions with time to failure
		if result.Score > 30 || result.TimeToFailure != "N/A" {
			predictions = append(predictions, result)
		}
	}
	
	return predictions
}

func (p *Predictor) analyzePodAdvanced(current collector.PodMetrics, history []collector.PodMetrics) PredictionResult {
	result := PredictionResult{
		PodName:         current.Name,
		PodNamespace:    current.Namespace,
		Risk:            "LOW",
		Issues:          []string{},
		Action:          "MONITOR",
		Confidence:      50,
		Score:           0,
		TimeToFailure:   "N/A",
		Trend:           "STABLE",
		MemoryLeakRate:  0,
		CPUGrowthRate:   0,
		PredictionHours: 0,
		Model:           "healer-v2-multisignal",
		WindowPoints:    len(history),
		Reason:          "",
		Signals:         []Signal{},
	}
	
	score := 0.0

	// Helper: baseline mean/std for a metric over history
	meanStd := func(vals []float64) (float64, float64) {
		if len(vals) == 0 {
			return 0, 0
		}
		m := 0.0
		for _, v := range vals {
			m += v
		}
		m /= float64(len(vals))
		v := 0.0
		for _, x := range vals {
			d := x - m
			v += d * d
		}
		if len(vals) > 1 {
			v /= float64(len(vals) - 1)
		}
		s := math.Sqrt(v)
		return m, s
	}

	addSignal := func(sig Signal) {
		result.Signals = append(result.Signals, sig)
		if sig.Score > 0 {
			score += sig.Score
		}
		if sig.Note != "" {
			result.Issues = append(result.Issues, fmt.Sprintf("%s: %s", sig.Name, sig.Note))
		}
	}

	// Collect history vectors
	cpuHist := make([]float64, 0, len(history))
	memHist := make([]float64, 0, len(history))
	restHist := make([]float64, 0, len(history))
	for _, h := range history {
		cpuHist = append(cpuHist, h.CPUPercent)
		memHist = append(memHist, h.MemPercent)
		restHist = append(restHist, float64(h.Restarts))
	}

	// Baseline anomaly signals (z-score-ish)
	cpuMean, cpuStd := meanStd(cpuHist)
	memMean, memStd := meanStd(memHist)
	if cpuStd < 0.5 {
		cpuStd = 0.5
	}
	if memStd < 0.5 {
		memStd = 0.5
	}

	cpuZ := (current.CPUPercent - cpuMean) / cpuStd
	memZ := (current.MemPercent - memMean) / memStd

	// CPU anomaly
	{
		sev := "info"
		sigScore := 0.0
		note := ""
		if cpuZ >= 4 {
			sev = "critical"
			sigScore = 35
			note = fmt.Sprintf("spike %.1fσ over baseline", cpuZ)
		} else if cpuZ >= 3 {
			sev = "warning"
			sigScore = 22
			note = fmt.Sprintf("elevated %.1fσ over baseline", cpuZ)
		} else if current.CPUPercent >= 15 {
			sev = "warning"
			sigScore = 12
			note = fmt.Sprintf("high absolute CPU %.1f%%", current.CPUPercent)
		}
		addSignal(Signal{
			Name:     "CPU",
			Current:  current.CPUPercent,
			Baseline: cpuMean,
			Delta:    current.CPUPercent - cpuMean,
			Score:    sigScore,
			Severity: sev,
			Note:     note,
		})
	}

	// Memory anomaly
	{
		sev := "info"
		sigScore := 0.0
		note := ""
		if memZ >= 4 {
			sev = "critical"
			sigScore = 35
			note = fmt.Sprintf("spike %.1fσ over baseline", memZ)
		} else if memZ >= 3 {
			sev = "warning"
			sigScore = 22
			note = fmt.Sprintf("elevated %.1fσ over baseline", memZ)
		} else if current.MemPercent >= 15 {
			sev = "warning"
			sigScore = 12
			note = fmt.Sprintf("high absolute memory %.1f%%", current.MemPercent)
		}
		addSignal(Signal{
			Name:     "Memory",
			Current:  current.MemPercent,
			Baseline: memMean,
			Delta:    current.MemPercent - memMean,
			Score:    sigScore,
			Severity: sev,
			Note:     note,
		})
	}

	// Restart spike signal
	{
		restMean, restStd := meanStd(restHist)
		if restStd < 0.5 {
			restStd = 0.5
		}
		restZ := (float64(current.Restarts) - restMean) / restStd
		sev := "info"
		sigScore := 0.0
		note := ""
		if current.Restarts >= 5 || restZ >= 3 {
			sev = "warning"
			sigScore = 18
			note = fmt.Sprintf("restart spike restarts=%d (%.1fσ)", current.Restarts, restZ)
		}
		addSignal(Signal{
			Name:     "Restarts",
			Current:  float64(current.Restarts),
			Baseline: restMean,
			Delta:    float64(current.Restarts) - restMean,
			Score:    sigScore,
			Severity: sev,
			Note:     note,
		})
	}

	// === 1. TREND & FORECAST SIGNALS (existing logic) ===
	// Keep existing trend modeling but treat it as weighted signals.
	if len(history) >= 5 {
		trend := p.calculateAdvancedTrend(history, current)
		result.MemoryLeakRate = trend.MemSlope
		result.CPUGrowthRate = trend.CPUSlope

		// CPU Growth Prediction (24-72 hour window)
		if trend.CPUSlope > 2 { // Growing >2% per hour
			hoursToFailure := (100 - current.CPUPercent) / trend.CPUSlope
			if hoursToFailure > 0 && hoursToFailure <= 72 {
				addSignal(Signal{
					Name:     "CPU Growth",
					Current:  trend.CPUSlope,
					Baseline: 0,
					Delta:    trend.CPUSlope,
					Score:    30,
					Severity: "warning",
					Note:     fmt.Sprintf("Growing %.1f%%/hour → will reach 100%% in %.1f hours", trend.CPUSlope, hoursToFailure),
				})
				result.TimeToFailure = fmt.Sprintf("%.1f hours (CPU overload)", hoursToFailure)
				result.PredictionHours = int(hoursToFailure)
			}
		}

		// Memory Leak Detection (most important!)
		if trend.MemSlope > 1 { // Growing >1% per hour
			hoursToFailure := (100 - current.MemPercent) / trend.MemSlope
			if hoursToFailure > 0 && hoursToFailure <= 72 {
				addSignal(Signal{
					Name:     "Memory Leak",
					Current:  trend.MemSlope,
					Baseline: 0,
					Delta:    trend.MemSlope,
					Score:    35,
					Severity: "critical",
					Note:     fmt.Sprintf("Growing %.1f%%/hour → OOM in %.1f hours", trend.MemSlope, hoursToFailure),
				})
				result.TimeToFailure = fmt.Sprintf("%.1f hours (Memory leak)", hoursToFailure)
				result.PredictionHours = int(hoursToFailure)
			}
		}
	}

	// === 2. FINALIZE RISK ASSESSMENT ===
	result.Score = math.Min(score, 100)

	// Derive a short human-readable reason from the strongest signals
	bestIdx := -1
	secondIdx := -1
	for i := range result.Signals {
		if bestIdx == -1 || result.Signals[i].Score > result.Signals[bestIdx].Score {
			secondIdx = bestIdx
			bestIdx = i
			continue
		}
		if secondIdx == -1 || result.Signals[i].Score > result.Signals[secondIdx].Score {
			secondIdx = i
		}
	}

	if bestIdx >= 0 && result.Signals[bestIdx].Score > 0 {
		s1 := result.Signals[bestIdx]
		if secondIdx >= 0 && result.Signals[secondIdx].Score > 0 {
			s2 := result.Signals[secondIdx]
			result.Reason = fmt.Sprintf("%s + %s", s1.Name, s2.Name)
		} else {
			result.Reason = s1.Name
		}
	}

	// Confidence: grows with window size and score tier
	windowBoost := 0
	if result.WindowPoints > 0 {
		windowBoost = int(math.Min(30, float64(result.WindowPoints)*3))
	}

	if result.Score >= 80 {
		result.Risk = "CRITICAL"
		result.Confidence = 70 + windowBoost
	} else if result.Score >= 60 {
		result.Risk = "HIGH"
		result.Confidence = 60 + windowBoost
	} else if result.Score >= 40 {
		result.Risk = "MEDIUM"
		result.Confidence = 50 + windowBoost
	} else if result.Score >= 20 {
		result.Risk = "LOW-MEDIUM"
		result.Confidence = 45 + windowBoost
	} else {
		result.Risk = "LOW"
		// Confidence will be refined as history grows; keep LOW risk at high confidence.
		result.Confidence = 70 + windowBoost
	}

	if result.Confidence > 99 {
		result.Confidence = 99
	}
	if result.Confidence < 25 {
		result.Confidence = 25
	}

	// Action selection: production-style prioritization based on top signal + risk
	// Keep existing action strings expected by the ActionEngine.
	if result.Risk == "CRITICAL" {
		if bestIdx >= 0 {
			s := result.Signals[bestIdx]
			if s.Name == "Memory Leak" || s.Name == "Memory" {
				result.Action = "RESTART_POD_URGENT"
			} else if s.Name == "CPU Growth" || s.Name == "CPU" {
				result.Action = "SCALE_UP_URGENT"
			} else if s.Name == "Restarts" {
				result.Action = "INVESTIGATE_RESTARTS"
			}
		}
	} else if result.Risk == "HIGH" {
		if bestIdx >= 0 {
			s := result.Signals[bestIdx]
			if s.Name == "Memory Leak" || s.Name == "Memory" {
				result.Action = "RESTART_POD"
			} else if s.Name == "CPU Growth" || s.Name == "CPU" {
				result.Action = "SCALE_UP"
			} else if s.Name == "Restarts" {
				result.Action = "INVESTIGATE_RESTARTS"
			} else {
				result.Action = "MONITOR_CLOSELY"
			}
		}
	} else if result.Risk == "MEDIUM" {
		result.Action = "MONITOR_CLOSELY"
	} else {
		result.Action = "MONITOR"
	}

	return result
}

func (p *Predictor) calculateAdvancedTrend(history []collector.PodMetrics, current collector.PodMetrics) TrendAnalysis {
	if len(history) < 3 {
		return TrendAnalysis{CPUTrend: "UNKNOWN", MemTrend: "UNKNOWN"}
	}
    
    // Calculate time span in hours (measurements every 30 seconds)
    timeSpan := float64(len(history)) * 0.5 / 60.0 // Convert to hours
    if timeSpan == 0 {
        timeSpan = 0.5 // Minimum 30 minutes
    }
    
    // Calculate trends using linear regression for better accuracy
    cpuSlope := p.calculateSlope(history, current, "cpu")
    memSlope := p.calculateSlope(history, current, "memory")
    
    trend := TrendAnalysis{
        CPUSlope:      cpuSlope,
        MemSlope:      memSlope,
        IsMemoryLeak:  memSlope > 1,
        IsCPUGrowing:  cpuSlope > 2,
    }
    
    // Classify trends
    if cpuSlope > 5 {
        trend.CPUTrend = "RISING_FAST"
    } else if cpuSlope > 2 {
        trend.CPUTrend = "RISING"
    } else if cpuSlope < -5 {
        trend.CPUTrend = "FALLING_FAST"
    } else if cpuSlope < -2 {
        trend.CPUTrend = "FALLING"
    } else {
        trend.CPUTrend = "STABLE"
    }
    
    if memSlope > 3 {
        trend.MemTrend = "RISING_FAST"
    } else if memSlope > 1 {
        trend.MemTrend = "RISING"
    } else if memSlope < -3 {
        trend.MemTrend = "FALLING_FAST"
    } else if memSlope < -1 {
        trend.MemTrend = "FALLING"
    } else {
        trend.MemTrend = "STABLE"
    }
    
    return trend
}

func (p *Predictor) calculateSlope(history []collector.PodMetrics, current collector.PodMetrics, resourceType string) float64 {
    if len(history) < 2 {
        return 0
    }
    
    // Simple linear regression to find slope (change per hour)
    n := float64(len(history) + 1)
    timeSpan := n * 0.5 / 60.0 // hours
    
    var values []float64
    for _, h := range history {
        if resourceType == "cpu" {
            values = append(values, h.CPUPercent)
        } else {
            values = append(values, h.MemPercent)
        }
    }
    
    if resourceType == "cpu" {
        values = append(values, current.CPUPercent)
    } else {
        values = append(values, current.MemPercent)
    }
    
    // Calculate slope using first and last values (simplified)
    if len(values) >= 2 {
        first := values[0]
        last := values[len(values)-1]
        return (last - first) / timeSpan
    }
    
    return 0
}

func (p *Predictor) detectPerformanceDegradation(history []collector.PodMetrics, current collector.PodMetrics) bool {
    if len(history) < 5 {
        return false
    }
    
    // Check for gradual increase in resource usage without spikes
    cpuIncreases := 0
    memIncreases := 0
    
    for i := 1; i < len(history); i++ {
        if history[i].CPUPercent > history[i-1].CPUPercent {
            cpuIncreases++
        }
        if history[i].MemPercent > history[i-1].MemPercent {
            memIncreases++
        }
    }
    
    // Performance degradation if >70% of measurements show increases
    degradationThreshold := float64(len(history)) * 0.7
    return float64(cpuIncreases) > degradationThreshold || float64(memIncreases) > degradationThreshold
}

func (p *Predictor) PrintPredictions(predictions []PredictionResult) {
    if len(predictions) == 0 {
        fmt.Printf("🟢 All pods healthy - no issues predicted\n\n")
        return
    }
    
    fmt.Printf("🔮 === AI SMART PREDICTIONS & FORECASTS ===\n")
    for _, pred := range predictions {
        riskIcon := "🟡"
        if pred.Risk == "CRITICAL" {
            riskIcon = "🔴"
        } else if pred.Risk == "HIGH" {
            riskIcon = "🟠"
        } else if pred.Risk == "MEDIUM" {
            riskIcon = "🟡"
        }
        
        fmt.Printf("%s Pod: %s/%s - Risk: %s (Score: %.1f, %d%% confidence)\n", 
            riskIcon, pred.PodNamespace, pred.PodName, pred.Risk, pred.Score, pred.Confidence)
        
        if pred.TimeToFailure != "N/A" {
            fmt.Printf("  ⏰ PREDICTION: Failure in %s\n", pred.TimeToFailure)
        }
        
        if pred.MemoryLeakRate > 1 {
            fmt.Printf("  🩸 Memory leak: +%.1f%%/hour\n", pred.MemoryLeakRate)
        }
        
        if pred.CPUGrowthRate > 2 {
            fmt.Printf("  📈 CPU growth: +%.1f%%/hour\n", pred.CPUGrowthRate)
        }
        
        if pred.Trend != "STABLE" {
            fmt.Printf("  📊 Trend: %s\n", pred.Trend)
        }
        
        for _, issue := range pred.Issues {
            fmt.Printf("  ⚠️  %s\n", issue)
        }
        
        fmt.Printf("  💡 AI Action: %s\n\n", pred.Action)
    }
    fmt.Printf("=======================================\n\n")
}
