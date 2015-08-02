package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/mattetti/m3u8Grabber/m3u8"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	m3u8.Debug = true
}

var (
	destPathFlag = flag.String("dest", "", "where the files are going to be stored.")
	configFlag   = flag.String("config", "config.json", "path to the config file.")
	config       *Config
	wsURL        = "http://pluzz.webservices.francetelevisions.fr/pluzz/liste/type/replay/rubrique/jeunesse/nb/200/debut/0"
	MaxRetries   = 4
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s \n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *destPathFlag == "" {
		flag.PrintDefaults()
		log.Println("Set the destination folder")
		os.Exit(1)
	}
	if _, err := os.Stat(*destPathFlag); err != nil {
		log.Println(err)
		os.Exit(1)
	}

	if _, err := os.Stat(*configFlag); err != nil {
		log.Println("Issue loading config file at path:", *configFlag)
		log.Println(err)
		os.Exit(1)
	}

	f, err := os.Open(*configFlag)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	config = &Config{}
	if err = json.NewDecoder(f).Decode(config); err != nil {
		log.Println("error decoding json")
		log.Fatal(err)
	}

	cmd := exec.Command("which", "ffmpeg")
	_, err = cmd.Output()
	if err != nil {
		log.Fatal("ffmpeg wasn't found on your system, it is required to convert video files.")
	}

	var w sync.WaitGroup
	stopChan := make(chan bool)
	m3u8.LaunchWorkers(&w, stopChan)

	response, err := http.Get(wsURL)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	var data wsResp
	if err := json.NewDecoder(response.Body).Decode(&data); err != nil {
		log.Fatal(err)
	}

	for i, em := range data.Response.Emissions {
		var title string
		episode := em.Episode
		season := em.Saison
		if season == "" {
			season = "00"
		} else if len(season) < 2 {
			season = "0" + season
		}
		if episode == "" {
			episode = em.IDDiffusion
		}
		filename := fmt.Sprintf("%s - S%sE%s - %s", em.Titre, season, episode, em.Soustitre)
		title = fmt.Sprintf("[%d] %s ", i, filename)

		if !config.shouldDownload(em.Titre) {
			log.Println(title, "not registered, skipping")
			continue
		}

		// skip if the file was already downloaded
		path := filepath.Join(*destPathFlag, em.Titre)
		mp4Output := filepath.Join(path, m3u8.CleanFilename(filename)+".mp4")
		if _, err := os.Stat(mp4Output); err == nil {
			log.Println("skipping download", mp4Output, "alreay exist!")
			continue
		}

		resp, err := http.Get(em.manifestURL())
		if err != nil {
			log.Printf("error crafting manifest url: %v\n", err)
			continue
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode > 299 {
			log.Fatal(fmt.Errorf("downloading m3u8 failed [%d] -\n%s", resp.StatusCode, body))
		}

		playlist := &M3u8{Content: body}
		s := playlist.HighestQualityStream()
		if s == nil {
			continue
		}
		log.Println(">> downloading", filename, "to", path)
		m3u8.DlChan <- &m3u8.WJob{Type: m3u8.ListDL, URL: s.URL, DestPath: path, Filename: filename}
		log.Printf("%s queued up for download\n", filename)
	}
	log.Println("waiting for all downloads to be done")
	w.Wait()
	log.Println("done processing TV shows")
	if err := os.RemoveAll(m3u8.TmpFolder); err != nil {
		log.Fatalf("failed to clean up tmp folder %s\n", m3u8.TmpFolder)
	}
}

type Config struct {
	Whitelist []string `json:"whitelist"`
}

func (c *Config) shouldDownload(name string) bool {
	if c == nil {
		return false
	}
	for i := 0; i < len(c.Whitelist); i++ {
		if c.Whitelist[i] == name {
			return true
		}
	}
	return false
}

type wsResp struct {
	Query    struct{} `json:"query"`
	Response response `json:"reponse"`
}

type response struct {
	Nb        int        `json:"nb"`
	Total     int        `json:"total"`
	Emissions []emission `json:"emissions"`
}

type infoOeuvre struct {
	CodeProgramme string  `xml:"code_programme"`
	Videos        []video `xml:"videos>video"`
}

type video struct {
	Format        string `xml:"format"`
	URL           string `xml:"url"`
	Statut        string `xml:"statut"`
	DateFinMandat string `xml:"date_fin_mandat"`
}

type emission struct {
	IDDiffusion       string `json:"id_diffusion"`
	NbVues            string `json:"nb_vues"`
	VolonteReplay     string `json:"volonte_replay"`
	Replay            string `json:"replay"`
	MandatDuree       string `json:"mandat_duree"`
	Etranger          string `json:"etranger"`
	URL               string `json:"url"`
	Soustitrage       string `json:"soustitrage"`
	Recurrent         string `json:"recurrent"`
	URLVideoSitemap   string `json:"url_video_sitemap"`
	TempsRestant      string `json:"temps_restant"`
	DureeReelle       string `json:"duree_reelle"`
	NbDiffusion       string `json:"nb_diffusion"`
	Image300          string `json:"image_300"`
	CsaCode           string `json:"csa_code"`
	ImageSmall        string `json:"image_small"`
	TitreProgramme    string `json:"titre_programme"`
	Format            string `json:"format"`
	CodeProgramme     string `json:"code_programme"`
	DateDiffusion     string `json:"date_diffusion"`
	Acteurs           string `json:"acteurs"`
	Episode           string `json:"episode"`
	ChaineID          string `json:"chaine_id"`
	Image200          string `json:"image_200"`
	Rubrique          string `json:"rubrique"`
	OasSitepage       string `json:"oas_sitepage"`
	ImageLarge        string `json:"image_large"`
	Invites           string `json:"invites"`
	Nationalite       string `json:"nationalite"`
	IDEmission        string `json:"id_emission"`
	AccrocheProgramme string `json:"accroche_programme"`
	Duree             string `json:"duree"`
	IDCollection      string `json:"id_collection"`
	ImageMedium       string `json:"image_medium"`
	Soustitre         string `json:"soustitre"`
	BureauRegional    string `json:"bureau_regional"`
	IDProgramme       string `json:"id_programme"`
	Realisateurs      string `json:"realisateurs"`
	Presentateurs     string `json:"presentateurs"`
	ExtensionImage    string `json:"extension_image"`
	URLImageRacine    string `json:"url_image_racine"`
	Accroche          string `json:"accroche"`
	Titre             string `json:"titre"`
	GenreSimplifie    string `json:"genre_simplifie"`
	Genre             string `json:"genre"`
	Image100          string `json:"image_100"`
	TsDiffusionUtc    string `json:"ts_diffusion_utc"`
	GenreFiltre       string `json:"genre_filtre"`
	Hashtag           string `json:"hashtag"`
	Saison            string `json:"saison"`
	CsaNomLong        string `json:"csa_nom_long"`
	ChaineLabel       string `json:"chaine_label"`

	// cache
	m3u8URL string
}

func (e *emission) manifestURL() string {
	if e == nil {
		return ""
	}
	if e.m3u8URL != "" {
		return e.m3u8URL
	}
	infoURL := "http://webservices.francetelevisions.fr/tools/getInfosOeuvre/v2/?catalogue=Pluzz&idDiffusion=" + e.IDDiffusion
	resp, err := http.Get(infoURL)
	if err != nil {
		log.Printf("error crafting manifest url: %v\n", err)
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Printf("%d for ", resp.StatusCode, infoURL)
		return ""
	}

	d := json.NewDecoder(resp.Body)
	var info Oeuvre
	if err := d.Decode(&info); err != nil {
		log.Printf("error parsing the oeuvre info for %s - %v\n", infoURL, err)
		return ""
	}

	if len(info.Videos) < 1 {
		return ""
	}

	for _, video := range info.Videos {
		if video.Format == "hls_v5_os" {
			e.m3u8URL = video.URL
			return e.m3u8URL
		}
	}
	return ""
}

type Diffusion struct {
	Timestamp int    `json:"timestamp"`
	DateDebut string `json:"date_debut"`
}

type Person struct {
	Nom       string   `json:"nom"`
	Prenom    string   `json:"prenom"`
	Fonctions []string `json:"fonctions"`
}

type Video struct {
	Format          string   `json:"format"`
	URL             string   `json:"url"`
	Statut          string   `json:"statut"`
	Drm             bool     `json:"drm"`
	Embed           bool     `json:"embed"`
	Geoblocage      []string `json:"geoblocage"`
	PlagesOuverture []struct {
		Debut     int         `json:"debut"`
		Fin       int         `json:"fin"`
		Direct    bool        `json:"direct"`
		Startover interface{} `json:"startover"`
	} `json:"plages_ouverture"`
}

type Oeuvre struct {
	ID                string      `json:"id"`
	RefSource         string      `json:"ref_source"`
	CodeProgramme     string      `json:"code_programme"`
	Titre             string      `json:"titre"`
	SousTitre         string      `json:"sous_titre"`
	Synopsis          string      `json:"synopsis"`
	Genre             string      `json:"genre"`
	GenrePluzz        string      `json:"genre_pluzz"`
	GenrePluzzAntenne string      `json:"genre_pluzz_antenne"`
	Type              string      `json:"type"`
	Episode           interface{} `json:"episode"`
	Saison            interface{} `json:"saison"`
	Diffusion         Diffusion   `json:"diffusion"`
	TexteDiffusions   string      `json:"texte_diffusions"`
	Duree             string      `json:"duree"`
	RealDuration      int         `json:"real_duration"`
	Image             string      `json:"image"`
	Chaine            string      `json:"chaine"`
	//Credit interface{} `json:"credit"`
	//Region interface{} `json:"region"`
	URLSite          string      `json:"url_site"`
	URLGuidetv       interface{} `json:"url_guidetv"`
	Personnes        []Person    `json:"personnes"`
	Videos           []Video     `json:"videos"`
	URLReference     string      `json:"url_reference"`
	Direct           interface{} `json:"direct"`
	IDAedra          string      `json:"id_aedra"`
	SemaineDiffusion interface{} `json:"semaine_diffusion"`
	Droit            struct {
		Type string `json:"type"`
		Csa  string `json:"csa"`
	} `json:"droit"`
	Subtitles []interface{} `json:"subtitles"`
	Sequences []interface{} `json:"sequences"`
	Lectures  struct {
		ID         interface{} `json:"id"`
		NbLectures int         `json:"nb_lectures"`
	} `json:"lectures"`
	LecturesGroupes      []interface{} `json:"lectures_groupes"`
	Votes                interface{}   `json:"votes"`
	Indexes              []interface{} `json:"indexes"`
	Ordre                interface{}   `json:"ordre"`
	TagOas               interface{}   `json:"tag_OAS"`
	IDEmissionPlurimedia int           `json:"id_emission_plurimedia"`
	Audiodescription     bool          `json:"audiodescription"`
	Spritesheet          interface{}   `json:"spritesheet"`
	IDTaxo               interface{}   `json:"id_taxo"`
}

/* M3u8 stuff to move to its own file */

// M3u8 stuff based on http://tools.ietf.org/html/draft-pantos-http-live-streaming
type M3u8 struct {
	Content  []byte
	parsed   bool
	streams  []*M3u8Stream
	segments []*M3u8Seg
}

func (m *M3u8) parse() error {
	if m == nil {
		return fmt.Errorf("can't parse a nil pointer")
	}
	scanner := bufio.NewScanner(bytes.NewBuffer(m.Content))
	var desc string
	var content string
	var isDesc bool
	for scanner.Scan() {
		line := scanner.Text()
		isDesc = strings.HasPrefix(line, "#EXT")
		if isDesc {
			desc = line
		} else {
			content = line
			// at this point we should have a desc and a content
			if strings.HasPrefix(desc, "#EXT-X-STREAM-INF:") {
				// we have a stream
				// #EXT-X-STREAM-INF:SUBTITLES="subs",PROGRAM-ID=1,BANDWIDTH=181000,RESOLUTION=256x144,CODECS="avc1.66.30, mp4a.40.2"
				// http://replayftv-vh.akamaihd.net/i/streaming-adaptatif_france-dom-tom/2015/S28/J1/124932599-559a236fa9309-,standard1,standard2,standard3,standard4,standard5,.mp4.csmil/index_0_av.m3u8?null=
				tmp := strings.Split(desc, ":")
				if len(tmp) < 2 {
					continue
				}
				stream := &M3u8Stream{URL: content}
				elements := strings.Split(tmp[1], ",")
				for _, el := range elements {
					kv := strings.Split(el, "=")
					switch kv[0] {
					case "BANDWIDTH":
						stream.Bandwidth, _ = strconv.Atoi(kv[1])
					case "RESOLUTION":
						wh := strings.Split(kv[1], "x")
						w, _ := strconv.Atoi(wh[0])
						h, _ := strconv.Atoi(wh[1])
						stream.Resolution = [2]int{w, h}
					case "CODECS":
						stream.Codecs = kv[1]
					}
				}
				m.streams = append(m.streams, stream)
			}

			if strings.HasPrefix(desc, "#EXTINF:") {
				// we have a segment
			}
		}

	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	// TODO: verify first line for #EXTM3U
	// TODO: process each line
	// if #EXT-X-STREAM-INF:
	// stream info
	// if #EXTINF:
	// segment
	m.parsed = true
	return nil
}

func (m *M3u8) Streams() []*M3u8Stream {
	if m == nil {
		return nil
	}

	if !m.parsed {
		if err := m.parse(); err != nil {
			log.Println("error parsing m3u8 content - %v", err)
			return nil
		}
	}
	return m.streams
}

func (m *M3u8) HighestQualityStream() *M3u8Stream {
	if m == nil {
		return nil
	}
	var topStream *M3u8Stream
	for _, s := range m.Streams() {
		if topStream == nil {
			topStream = s
			continue
		}
		// compares on width only, resolution + bandwidth might be even better
		if s.Resolution[0] > topStream.Resolution[0] {
			topStream = s
		}
	}
	return topStream
}

type M3u8Stream struct {
	Bandwidth  int
	Resolution [2]int
	Codecs     string
	URL        string
	Segments   []*M3u8Seg
}

func download(s *M3u8Stream, destPath, destFilename string) error {
	if s == nil {
		return fmt.Errorf("can't download using a nil stream")
	}
	return m3u8.DownloadM3u8ContentWithRetries(s.URL, destPath, destFilename, "", "", 0)
}

type M3u8Seg struct {
	Order    int
	Duration string // TODO convert in time.Duration
	URL      string
}
