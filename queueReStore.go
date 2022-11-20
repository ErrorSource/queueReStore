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
	"strings"
	// "os/exec"
)

// caller:
//  # cd queueReStore/
//  # go build queueReStore.go && ./queueReStore -mode store|restore

// application version
const AppVersion = "1.0.0"

var Config config
// queue item structure
type config struct {
	APIUrl     string `yaml:"otAPIUrl"`
	OTUid      int    `yaml:"otUid"`
	OTGid      int    `yaml:"otGid"`
	ActPosTrgt string `yaml:"actPosTargetPath"`
	PlsTarget  string `yaml:"plsTargetPath"`
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


func main() {
	version := flag.Bool("version", false, "show version and exit")
	var mode string
	flag.StringVar(&mode, "mode", "", "[store|restore] queue")
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
			fmt.Println(fmt.Sprintf("ownTone-queue successfully written to:\n    '%s'", Config.PlsTarget))
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
			fmt.Println(fmt.Sprintf("ownTones actual queue position stored to:\n    '%s'\n", Config.ActPosTrgt))
			fmt.Println("Success!\n")
		} else {
			fmt.Println("Something went wrong!")
			os.Exit(1)
		}

		// call /usr/local/bin/queuePosStore
		// if err := exec.Command("/usr/local/bin/queuePosStore").Start(); err != nil {
		// 	errors.New("Can not execute '/usr/local/bin/queuePosStore'!")
		// }
	}

	if (mode == "restore") {
		// curl -X GET "http://localhost:3689/api/library/playlists" → json-Objekt
		// { "id": 141, "name": "_beforeShairport", "path": "\/media\/Playlists\/_beforeShairport.m3u"...
		// id extrahieren
		// curl -X POST "http://localhost:3689/api/queue/items/add?uris=library:playlist:${playlistID}&clear=true&shuffle=false"
		// curl -X POST "http://localhost:3689/api/queue/items/add?uris=library:playlist:17&clear=true&shuffle=false&playback=start&playback_from_position=10"
		// if ItemProgressMS kleiner 5sek → bei 0 anfangen; sonst -3sec = -3000ms starten und dann stoppen
		// auf Zeit-Position springen
		// curl -X PUT "http://localhost:3689/api/player/seek?position_ms=20000"

		// read
		player, err := readActPosFile()
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Printf("%+v\n", player)

		// delete plsTargetPath file
		if _, err := os.Stat(Config.PlsTarget); err == nil {
			if err := os.Remove(Config.PlsTarget); err != nil {
				fmt.Println(fmt.Sprintf("Can not delete file '%s'! Continuing anyway...", Config.PlsTarget))
			}
		} else if errors.Is(err, os.ErrNotExist) {
		  // path/to/whatever does *not* exist
			fmt.Println(fmt.Sprintf("Playlist file '%s' does not exists! Continuing anyway...", Config.PlsTarget))
		}

		fmt.Println("Not definde yet!\n")
		os.Exit(1)
	}
}
