# queueReStore
Store and restore OwnTones actual queue.

Useful, if you use OwnTone combined with shairport-sync (piped output). Queue would be "cleared", if stream stops. So queue can be stored before shairport-sync pipes to OwnTone and restored, after playback stops.

### Configuration

Create a YAML-config file in _/etc/queueReStore.yml_:

````yaml
# queueReStore configurations

logFile:          '/var/log/queueReStore.log'

# local websrvice URL of owntone
otAPIUrl:         'http://localhost:3689/api'

# uid of user 'owntone'
otUid:            115

# gui of group 'audio'
otGid:            29

# target file for storing actual track info (and position in queue)
actPosTargetPath: '/media/.queueStoredPos'

# target playlist-file for (temporarily) storing actual queue
plsTargetPath:    '/media/usbstick/Music/Playlists/_queueReStore.m3u'

# shairport-snyc pipe (FIFO-file); leave EMPTY to disable check
shrPrtPipePath:   '/mnt/ramdisk/shairport/shairport'
````

### Usage

````bash
Usage of ./queueReStore:
  -mode string
        [store|restore] queue
  -quiet
        write all output to log file only
  -version
        show version and exit

Example: ./queueReStore -quiet -mode store
````
