package main

import (
	"os"
	"encoding/json"
	"gopkg/yaml"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	// "os/exec"
)

// caller:
//  # cd queueReStore/
//  # go build queueReStore.go && ./queueReStore -mode store|restore

// application version
const AppVersion = "1.0.0"

var quiet bool

var Config config
// queue item structure
type config struct {
	APIUrl     string `yaml:"otAPIUrl"`
	OTUid      int    `yaml:"otUid"`
	OTGid      int    `yaml:"otGid"`
	ActPosTrgt string `yaml:"actPosTargetPath"`
	PlsTarget  string `yaml:"plsTargetPath"`
	SPPipePath string `yaml:"shrPrtPipePath"`
}

// queue item structure
type queueItem struct {
	ItemId   int    `json:"id"`
	Position int    `json:"position"`
	TrackId  int    `json:"track_id"`
	Artist   string `json:"artist"`
	Title    string `json:"title"`
	FilePath string `json:"path"`
	Uri      string `json:"uri"`
}
type queueObj struct {
	Version  int `json:"version"`
	Count    int `json:"count"`
	Items    []queueItem `json:"items"`
}

// player info structure
type playerInfo struct {
	State          string `json:"state"`
	RepeatMode     string `json:"repeat"`
	consumeMode    bool   `json:"consume"`
	ShuffleMode    bool   `json:"shuffle"`
	Volume         int    `json:"volume"`
	ItemId         int    `json:"item_id"`
	ItemLengthMS   int    `json:"item_length_ms"`
	ItemProgressMS int    `json:"item_progress_ms"`
	Position       int    
	TrackId        int    
	Uri            string ""
}

// playlist structure
type plsItem struct {
	ItemId   int    `json:"id"`
	Name     string `json:"name"`
	Uri      string `json:"uri"`
}

type plsObj struct {
	Total  int `json:"total"`
	Items  []plsItem `json:"items"`
}

func readConfig() {
	configData, err := ioutil.ReadFile("/etc/queueReStore.yml")
	if err != nil {
		fmt.Println("Can not read config-file!")
		log.Fatalln(err)
	}

	// parse []byte directly to the global variable 'Config'
	if err := yaml.Unmarshal(configData, &Config); err != nil {
		fmt.Println("Can not unmarshal config YAML!")
		log.Fatalln(err)
	}
}

// make a GET request from source url and load data
// return readed bytes or error
func makeRequest(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, errors.New("No response from request!")
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New("Cannot read data from request!")
	}

	if err := resp.Body.Close(); err != nil {
		return nil, errors.New("Cannot close data-stream!")
	}
	return data, nil
}

// convert downloaded data bytes (json) to list of queue items
func convertToQueueStruct(jsonData []byte) ([]queueItem, error) {
	var queueItems []queueItem
	
	// parse []byte to the go struct pointer
	var jsonObj queueObj
	if err := json.Unmarshal(jsonData, &jsonObj); err != nil {
		return nil, errors.New("Can not unmarshal JSON!")
	}
	
	for _, item := range jsonObj.Items {
		queueItems = append(queueItems, item)
	}

	if len(queueItems) == 0 {
		return nil, errors.New("No queue items found in data!")
	}

	return queueItems, nil
}

func writePlsFile(queueItems []queueItem) (bool, error) {
	var plsContent string
	// header
	plsContent = "#EXTM3U\n"

	// tracks
	for _, item := range queueItems {
		// abort immediately, if FilePath is shairport-sync pipe path
		if (item.FilePath == Config.SPPipePath) {
			return false, errors.New("shairport-sync Playlist detected! Won't store! Aborting...")
		}
		// track info
		plsContent = plsContent + fmt.Sprintf("#EXTINF:%d, %s - %s\n", item.TrackId, item.Artist, item.Title)
		// track path
		plsContent = plsContent + fmt.Sprintf("%s\n", item.FilePath)
	}
	// remove last newline
	plsContent = strings.TrimSuffix(plsContent, "\n")

	// write queue-content to file
	if err := os.WriteFile(Config.PlsTarget, []byte(plsContent), 0644); err != nil {
		return false, errors.New("Can not write m3u-file!")
	}
	// set ownership to owntone:audio
	if err := os.Chown(Config.PlsTarget, Config.OTUid, Config.OTGid); err != nil {
		return false, errors.New("Can not set ownership of m3u-file!")
	}

	return true, nil
}

// convert downloaded data bytes (json) to list of player info items
// add trackId, position and uri from queue-data, because this data is not available in player info
func convertToPlayerStruct(jsonData []byte, queueItems []queueItem) (*playerInfo, error) {
	// parse []byte to the go struct pointer
	var jsonObj playerInfo
	if err := json.Unmarshal(jsonData, &jsonObj); err != nil {
		return nil, errors.New("Can not unmarshal JSON!")
	}

	// search for item in queue
	for idQElem := range queueItems {
		if queueItems[idQElem].ItemId == jsonObj.ItemId {
			// add missing data to player info
			jsonObj.Position = queueItems[idQElem].Position + 1
			jsonObj.TrackId = queueItems[idQElem].TrackId
			jsonObj.Uri = queueItems[idQElem].Uri
			break
		}
	}

	return &jsonObj, nil
}

func writeActPosFile(player *playerInfo) (bool, error) {
	actPosFileString, _ := json.MarshalIndent(player, "", " ")
 
	// write player info content to file
	if err := ioutil.WriteFile(Config.ActPosTrgt, actPosFileString, 0644); err != nil {
		fmt.Println("Can not write actPos-file!")
		fmt.Println(Config.APIUrl)
		log.Fatalln(err)
	}

	// set ownership to owntone:audio
	if err := os.Chown(Config.ActPosTrgt, Config.OTUid, Config.OTGid); err != nil {
		return false, errors.New("Can not set ownership of actPos-file!")
	}

	return true, nil
}

func readActPosFile() (*playerInfo, error) {
	// read actPos file
	actPosFile, err := ioutil.ReadFile(Config.ActPosTrgt)
	if err != nil {
		return nil, errors.New("Can not read actPost-file!")
	}

	// parse []byte to the go struct pointer
	var jsonObj playerInfo
	if err := json.Unmarshal([]byte(actPosFile), &jsonObj); err != nil {
		return nil, errors.New("Can not unmarshal JSON!")
	}
	
	return &jsonObj, nil
}

// convert downloaded data bytes (json) to list of playlist items
func getOnwPlaylistUri(jsonData []byte) (string, error) {
	// parse []byte to the go struct pointer
	var jsonObj plsObj
	if err := json.Unmarshal(jsonData, &jsonObj); err != nil {
		return "", errors.New("Can not unmarshal JSON!")
	}
	
	var ownPlsUri string
	if (jsonObj.Total > 0) {
		for _, item := range jsonObj.Items {
			// search for playlist with name '_queueReStore'
			if (item.Name == "_queueReStore") {
				ownPlsUri = item.Uri
			}
		}
	}

	if (ownPlsUri == "") {
		return "", errors.New("Queue '_queueReStore.m3u' not found in data!")
	}

	return ownPlsUri, nil
}

func loadPlayistAndPosition(trgtPlsUri string, trgtPos int, shfflMode bool) (bool, error) {
	// append params in URL as well! the url.Values are not in charge here, but for the record...
	var loadReq string = Config.APIUrl+"/queue/items/add?uris="+trgtPlsUri+"&clear=true&shuffle="+fmt.Sprintf("%t", shfflMode)+"&playback=start&playback_from_position="+fmt.Sprintf("%v", (trgtPos - 1))
	// this has to be a POST-request!
	postData := url.Values{
		"uris":                   { trgtPlsUri },
		"clear":                  { "true" },
		"shuffle":                { fmt.Sprintf("%t", shfflMode) },
		"playback":               { "start" },
		"playback_from_position": { fmt.Sprintf("%v", (trgtPos - 1)) },
	}
	if _, err := http.PostForm(loadReq, postData); err != nil {
		return false, errors.New("Can not load stored playlist!")
	}

	// immediately send pause command
	pauseReq, _ := http.NewRequest("PUT", Config.APIUrl+"/player/pause", nil)
	client := &http.Client{}
	if _, err := client.Do(pauseReq); err != nil {
		fmt.Println("Can send 'pause' after stored playlist loaded! Continuing anyway...")
	}

	return true, nil
}


func main() {
	version := flag.Bool("version", false, "show version and exit")
	var mode string
	flag.StringVar(&mode, "mode", "", "[store|restore] queue")
	flag.BoolVar(&quiet, "quiet", false, "no output to stdout")
	flag.Parse()
	
	if *version {
		fmt.Println(AppVersion)
		os.Exit(0)
	}

	//if len(flag.Args()) < 1 {
	if (mode == "" || (mode != "store" && mode != "restore")) {
		flag.Usage()
		log.Fatalln("-mode must be set to [store|restore]!")
	}

	// read config from file '/etc/queueReStore.yml'
	readConfig()

	if (mode == "store") {
		//data, err := makeRequest(flag.Args()[len(flag.Args())-1])
		data, err := makeRequest(Config.APIUrl+"/queue")
		if err != nil {
			log.Fatalln(err)
		}

		queue, err := convertToQueueStruct(data)
		if err != nil {
			log.Fatalln(err)
		}

		success, err := writePlsFile(queue)
		if err != nil {
			log.Fatalln(err)
		}

		if (success == true) {
			if (quiet == false) {
				fmt.Println(fmt.Sprintf("ownTone-queue successfully written to:\n    '%s'", Config.PlsTarget))
			}
		} else {
			fmt.Println("Something went wrong!\n")
			os.Exit(1)
		}

		// get actual player info
		data, err = makeRequest(Config.APIUrl+"/player")
		if err != nil {
			log.Fatalln(err)
		}

		// covert data to struct
		player, err := convertToPlayerStruct(data, queue)
		if err != nil {
			log.Fatalln(err)
		}

		// store player info to .queue.storedPos file
		success, err = writeActPosFile(player)
		if err != nil {
			log.Fatalln(err)
		}

		if (success == true) {
			if (quiet == false) {
				fmt.Println(fmt.Sprintf("ownTones actual queue position stored to:\n    '%s'\n", Config.ActPosTrgt))
				fmt.Println("Success!\n")
			}
		} else {
			fmt.Println("Something went wrong!")
			os.Exit(1)
		}
	}

	if (mode == "restore") {
		// read
		player, err := readActPosFile()
		if err != nil {
			log.Fatalln(err)
		}

		data, err := makeRequest(Config.APIUrl+"/library/playlists")
		if err != nil {
			log.Fatalln(err)
		}
		
		ownPlsUri, err := getOnwPlaylistUri(data)
		if err != nil {
			log.Fatalln(err)
		}

		// store player info to .queue.storedPos file
		success, err := loadPlayistAndPosition(ownPlsUri, player.Position, player.ShuffleMode)
		if err != nil {
			log.Fatalln(err)
		}

		// delete plsTargetPath file
		if _, err := os.Stat(Config.PlsTarget); err == nil {
			if err := os.Remove(Config.PlsTarget); err != nil {
				fmt.Println(fmt.Sprintf("Can not delete file '%s'! Continuing anyway...", Config.PlsTarget))
			}
		} else if errors.Is(err, os.ErrNotExist) {
			// path does *not* exist
			fmt.Println(fmt.Sprintf("Playlist file '%s' does not exists! Continuing anyway...", Config.PlsTarget))
		}

		// delete actPosTargetPath file
		if _, err := os.Stat(Config.ActPosTrgt); err == nil {
			if err := os.Remove(Config.ActPosTrgt); err != nil {
				fmt.Println(fmt.Sprintf("Can not delete file '%s'! Continuing anyway...", Config.ActPosTrgt))
			}
		} else if errors.Is(err, os.ErrNotExist) {
			// path does *not* exist
			fmt.Println(fmt.Sprintf("Actual position file '%s' does not exists! Continuing anyway...", Config.ActPosTrgt))
		}

		if (success == true) {
			if (quiet == false) {
				fmt.Println(fmt.Sprintf("Restored playlist and jumped to track num: '%s'\n", fmt.Sprintf("%v", player.Position)))
				fmt.Println("Success!\n")
			}
		} else {
			fmt.Println("Something went wrong!")
			os.Exit(1)
		}
	}
}
