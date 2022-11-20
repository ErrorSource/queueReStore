# queueReStore
Store and restore OwnTones actual queue.

Useful, if you use OwnTone combined with shairport-sync (piped output). Queue would be "cleared", if stream stops. So queue can be stored before shairport-sync pipes to OwnTone and restored, after playback stops.

**ATTENTION**: At the moment **settings** (urls, paths, uid, gui) have to be adjusted in queueReStore.go itself. Config-file will be added later...

````bash
Usage of ./queueReStore:
  -mode string
        [store|restore] queue
  -version
        show version and exit

Example: ./queueReStore -mode store
