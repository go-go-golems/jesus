package web

import (
	"net/http"

	"github.com/go-go-golems/jesus/pkg/engine"
)

// HandleDynamicRoute processes requests for JavaScript-registered handlers
func HandleDynamicRoute(jsEngine *engine.Engine, w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	method := r.Method

	// Check for registered HTTP handler
	if handler, exists := jsEngine.GetHandler(method, path); exists {
		done := make(chan error, 1)
		job := engine.EvalJob{
			Handler: handler,
			W:       w,
			R:       r,
			Done:    done,
		}

		jsEngine.SubmitJob(job)

		// Wait for completion
		<-done
		return
	}

	// Check for registered file handler
	if fileHandler, exists := jsEngine.GetFileHandler(path); exists {
		done := make(chan error, 1)
		job := engine.EvalJob{
			Handler: &engine.HandlerInfo{Fn: fileHandler},
			W:       w,
			R:       r,
			Done:    done,
		}

		jsEngine.SubmitJob(job)

		// Wait for completion
		<-done
		return
	}

	// No handler found
	http.NotFound(w, r)
}
