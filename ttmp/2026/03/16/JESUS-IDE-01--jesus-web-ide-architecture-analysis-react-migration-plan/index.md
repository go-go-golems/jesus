---
Title: Jesus Web IDE Architecture Analysis & React Migration Plan
Ticket: JESUS-IDE-01
Status: active
Topics:
    - javascript
    - architecture
    - review
    - refactor
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/jesus/cmd/serve.go
      Note: CLI serve command — startup and server launch
    - Path: pkg/api/execute.go
      Note: Core /v1/execute endpoint bridging HTTP to engine
    - Path: pkg/engine/engine.go
      Note: Goja runtime
    - Path: pkg/web/handlers.templ.go
      Note: Page handlers and static file serving
    - Path: pkg/web/routes.go
      Note: Router setup for both admin and JS servers
    - Path: pkg/web/static/js/app.js
      Note: Frontend JSPlaygroundApp class — all client-side logic
    - Path: pkg/web/templates/playground.templ
      Note: Playground page Templ template
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-16T08:01:50.743498275-04:00
WhatFor: ""
WhenToUse: ""
---


# Jesus Web IDE Architecture Analysis & React Migration Plan

Document workspace for JESUS-IDE-01.
