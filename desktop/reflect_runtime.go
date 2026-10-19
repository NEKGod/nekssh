package main

import "github.com/wailsapp/wails/v2/pkg/runtime"

func emitReflectEvent(a *App, event, id, message string) {
	runtime.EventsEmit(a.ctx, event, id, message)
}
