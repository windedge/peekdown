package shell

import (
	"context"
	"os"
	"time"

	"github.com/egoist/mygo"
)

// Auto refresh: one mygo.WatchFiles per open tab's file, as mygo's own
// document model does. Watching the file rather than its directory costs
// the same whatever else the directory holds, so a folder of thousands
// of files cannot reach the watch's 8192-entry limit and no Ignore list
// is needed.
//
// Threading: the main thread alone starts and stops the watcher, mutates
// the tabs and snapshots their paths; each snapshot reaches the loop
// through watchKick, and every hit the watcher sees lands back on the
// main thread (watchHits drained by ApplyWatchReloads, the repaint
// through mygo.RunOnMain), the only place the document model may change
// under a live view. Nothing blocks on the main thread from the watcher
// side, so a shutdown can never wait on a queued reload.

// watchHits carries the paths the watcher saw change, for the next
// frame to reload on the main thread; a full channel drops the hit,
// the next change re-raising it.
const watchHitsDepth = 16

// reloadTab rereads tab's file into its model and drops the view's
// stale state: the selection, the overlays and the shaped text name
// the old document. The list keeps its scroll offset, so the reading
// place survives. Main thread only. It reports whether the content
// changed.
func (s *Shell) reloadTab(tab *Tab) bool {
	if tab == nil || tab.Path == "" {
		return false
	}
	data, err := os.ReadFile(tab.Path)
	if err != nil {
		return false
	}
	if tab.Model != nil && string(data) == tab.Model.Source() {
		return false
	}
	if err := tab.Model.Set(string(data)); err != nil {
		return false
	}
	tab.State.ClearSelection()
	tab.State.Matches = nil
	tab.State.Current = -1
	return true
}

// ApplyWatchReloads reloads every tab whose file the watcher saw
// change, at the head of the frame before the view reads the model.
// Main thread only; the host calls it from its view function.
func (s *Shell) ApplyWatchReloads() {
	for s.watchHits != nil {
		select {
		case path := <-s.watchHits:
			if i := s.tabIndex(path); i >= 0 {
				s.reloadTab(&s.Tabs[i])
			}
		default:
			return
		}
	}
}

// tabPaths snapshots the open tabs' paths. Main thread only.
func (s *Shell) tabPaths() []string {
	paths := make([]string, len(s.Tabs))
	for i := range s.Tabs {
		paths[i] = s.Tabs[i].Path
	}
	return paths
}

// syncWatch pokes the watcher loop after the tabs changed, so its watch
// set follows them; with auto refresh off it just stops the loop.
func (s *Shell) syncWatch() {
	if !s.Saved.AutoRefresh {
		s.stopWatch()
		return
	}
	if s.watchCancel == nil {
		s.startWatch()
		return
	}
	s.kickWatch()
}

// kickWatch hands the loop the current paths, dropping a snapshot still
// pending: the loop syncs with the latest set either way, and the one
// slot means the send never blocks the frame. Main thread only.
func (s *Shell) kickWatch() {
	paths := s.tabPaths()
	select {
	case <-s.watchKick:
	default:
	}
	s.watchKick <- paths
}

// applyAutoRefresh aligns the watcher with the auto_refresh setting.
// Called at startup and from Apply, so the settings dialog's switch
// starts and stops it with everything else the dialog saves.
func (s *Shell) applyAutoRefresh() {
	if s.Saved.AutoRefresh && len(s.Tabs) > 0 {
		s.startWatch()
		return
	}
	s.stopWatch()
}

// startWatch spawns the watcher goroutine unless one already runs. It
// keeps one watch alive per open tab's file and recreates them as tabs
// come and go, until the shell stops it or auto refresh turns off.
func (s *Shell) startWatch() {
	if s.watchCancel != nil || !s.Saved.AutoRefresh {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.watchCancel, s.watchDone = cancel, make(chan struct{})
	s.watchKick = make(chan []string, 1)
	hits := make(chan string, watchHitsDepth)
	s.watchHits = hits
	go s.watchLoop(ctx, hits, s.tabPaths())
}

// stopWatch cancels the watcher goroutine and waits for its watches to
// release.
func (s *Shell) stopWatch() {
	if s.watchCancel == nil {
		return
	}
	s.watchCancel()
	s.watchCancel = nil
	if s.watchDone != nil {
		<-s.watchDone
		s.watchDone = nil
	}
	s.watchHits = nil
}

// watchLoop is the watcher goroutine: it keeps the watches of the open
// tabs' files in step until ctx ends, taking each new set of paths from
// the main thread's kick. A watch that ends early (its file or directory
// went away, the entry limit) is restarted with the next sync, which a
// tab change or a reopen triggers.
func (s *Shell) watchLoop(ctx context.Context, hits chan<- string, paths []string) {
	defer close(s.watchDone)
	watches := map[string]*mygo.FileWatcher{}
	defer func() {
		for _, w := range watches {
			w.Close()
		}
	}()
	for {
		// Sync the watch set with the paths the main thread last sent.
		live := map[string]bool{}
		for _, path := range paths {
			if path == "" || live[path] {
				continue
			}
			live[path] = true
			if watches[path] != nil {
				continue
			}
			w, err := mygo.WatchFiles(ctx, path, mygo.FileWatchOptions{
				Debounce: watchDebounce,
			})
			if err != nil {
				// The file is gone; it may come back with the next sync.
				continue
			}
			watches[path] = w
			go s.pumpWatch(ctx, w, hits, path)
		}
		for path, w := range watches {
			if !live[path] {
				w.Close()
				delete(watches, path)
			}
		}
		select {
		case <-ctx.Done():
			// The pumps close their watches; wait for the handles gone.
			for _, w := range watches {
				<-w.Done()
			}
			return
		case paths = <-s.watchKick:
		}
	}
}

// pumpWatch applies one watch's batches: a change to the tab's file
// hands the path to the next frame, which reloads the tab on the main
// thread and repaints. It closes the watch when it ends, so watchLoop's
// next sync starts a fresh one.
func (s *Shell) pumpWatch(ctx context.Context, w *mygo.FileWatcher, hits chan<- string, path string) {
	for {
		events, err := w.Next(ctx)
		if err != nil {
			w.Close()
			return
		}
		// A file watch reports the file itself: "." for a write, or a
		// remove of "." when the file or its directory went away. A hit
		// for a tab the user closed meanwhile reloads nothing: the main
		// thread looks the path up and finds no tab.
		changed := false
		for _, e := range events {
			if e.Op != mygo.FileWrite || e.IsDir {
				continue
			}
			select {
			case hits <- path:
			default:
			}
			changed = true
		}
		if !changed {
			continue
		}
		mygo.RunOnMain(func() {
			if s.OnWatchReload != nil {
				s.OnWatchReload()
			}
		})
	}
}

// watchDebounce waits out editors' bursts of writes; mygo's default is
// 50 ms, which reloads mid-save. The document watch in mygo uses 100 ms.
const watchDebounce = 100 * time.Millisecond
