package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/nektro/go-util/sqlite"
	"github.com/nektro/go-util/util"
)

//
//

type WatchedFile struct {
	ID   int    `json:"id"`
	Path string `json:"path" sqlite:"text"`
	Name string `json:"name" sqlite:"text"`
	URL  string `json:"url"`
}

func scanFile(rows *sql.Rows) WatchedFile {
	var v WatchedFile
	rows.Scan(&v.ID, &v.Path, &v.Name)
	return v
}

//
//

var (
	watcher *fsnotify.Watcher
)

func initFsWatcher() {
	// creates a new file watcher
	watcher, _ = fsnotify.NewWatcher()
	database.CreateTableStruct("files", WatchedFile{})

	if err := filepath.Walk(config.Root, wWatchDir); err != nil {
		util.LogError(err)
	}

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				// util.Log("fsnotify", "event", event.Name, event.Op.String())
				r0 := strings.TrimPrefix(event.Name, config.Root)
				r1 := strings.Replace(r0, string(filepath.Separator), "/", -1)
				switch event.Op {
				case fsnotify.Rename, fsnotify.Remove:
					if sqlite.QueryHasRows(database.QueryPrepared(false, "select * from files where path = ?", r1)) {
						database.QueryPrepared(true, "delete from files where path = ?", r1)
					} else {
						r2 := r1 + "/"
						database.QueryPrepared(true, "delete from files where substr(path,1,length(?)) = ?", r2, r2)
					}
					util.Log("[file-index-del]", r1)
				case fsnotify.Create:
					f, _ := os.Stat(event.Name)
					if !f.IsDir() {
						n := f.Name()
						i := database.QueryNextID("files")
						database.QueryPrepared(true, "insert into files values (?, ?, ?)", i, r1, n)
						util.Log("[file-index-add]", r1)
					} else {
						if err := filepath.Walk(event.Name, wWatchDir); err != nil {
							util.LogError(err)
						}
					}
				}
			case err := <-watcher.Errors:
				util.LogError("[fsnotify]", err)
			}
		}
	}()
}

func wWatchDir(path string, fi os.FileInfo, err error) error {
	if fi.IsDir() {
		return watcher.Add(path)
	}
	wAddFile(strings.TrimPrefix(path, config.Root), fi.Name())
	return nil
}

func wAddFile(path string, name string) {
	pth := strings.Replace(path, string(filepath.Separator), "/", -1)
	if sqlite.QueryHasRows(database.QueryPrepared(false, "select * from files where path = ?", pth)) {
		return
	}
	id := database.QueryNextID("files")
	database.QueryPrepared(true, "insert into files values (?, ?, ?)", id, pth, name)
	util.Log("[file-index-add]", pth)
}
