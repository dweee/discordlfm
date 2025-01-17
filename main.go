package main

import (
	"errors"
	"flag"
	"bufio"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/shkh/lastfm-go/lastfm"
	"log"
	"os"
	"sync"
	"time"
	"unicode"
	"strings"
)

const (
	VERSION_MAJOR = 1
	VERSION_MINOR = 0
	VERSIN_PATCH  = 0
)

var (
	VersionString = fmt.Sprintf("%d.%d.%d", VERSION_MAJOR, VERSION_MINOR, VERSIN_PATCH)
)

var (
	flagDiscordToken   string
	flagLFMAPIKey      string
	flagLFMUsername    string
	flagNoSong         string
	flagNoSongDuration int
)

func init() {
	flag.StringVar(&flagDiscordToken, "t", "", "Discord token")
	flag.StringVar(&flagLFMAPIKey, "l", "", "Last.fm api key")
	flag.StringVar(&flagLFMUsername, "u", "", "Last.fm username")
	flag.StringVar(&flagNoSong, "g", "Silence", "Game to set to if there hasn't been a new song for a while")
	flag.IntVar(&flagNoSongDuration, "n", 60*10, "Number of seconds without a new song for it to be considered nothing.")
	flag.Parse()
}

func main() {
	log.Println("Starting up... v" + VersionString)

	if flagDiscordToken == "" {
		fatal("No discord token specified")
	}

	if flagLFMAPIKey == "" {
		fatal("No lastfm api key specified")
	}

	if flagLFMUsername == "" {
		fatal("No last.fm username specified")
	}

	// Setup lastfm
	lfm := lastfm.New(flagLFMAPIKey, "")

	// Setup discord
	dsession, err := discordgo.New(flagDiscordToken)
	if err != nil {
		fatal("Error creating discord session:", err)
	}

	var wg sync.WaitGroup
	dsession.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		wg.Done()
	})
	wg.Add(1)

	err = dsession.Open()
	if err != nil {
		fatal("Error opening discord ws conn:", err)
	}

	wg.Wait()

	log.Println("Ready received! Ctrl-c to stop.")
	run(dsession, lfm)
}

func redactVowels(s string) string{
	vowels := []byte{'a','e','i','o','u'}
	var res strings.Builder

	first := true
	for i := range s{
		found := false
		if first{
			for _,v := range vowels{
				if byte(unicode.ToLower(rune(s[i])))==v {
					res.WriteString("*")
					found=true
					first=false
					break
				}
			}
		}
		if !found{
			res.WriteByte(s[i])
		}

	}
	return res.String()
}

func censure(s string) string{
    for i := 0; i < len(s); i++ {
        if s[i] > unicode.MaxASCII {
            return s
        }
    }
	fi, err := os.Open("swears.txt")
	if err != nil{
		panic(err)
	}

	scanner:=bufio.NewScanner(fi)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan(){
		swear := scanner.Text()
		strings.ReplaceAll(swear,"\n","")
		for strings.Contains(strings.ToLower(s),swear) {
			idx := strings.Index(strings.ToLower(s),swear)
			s = s[:idx] + redactVowels(s[idx:idx+len(swear)]) + s[idx+len(swear):]
		}
	}
	if err:= scanner.Err(); err!= nil{
		panic(err)
	}
	fi.Close()
    return s
}

func run(s *discordgo.Session, lfm *lastfm.Api) {
	// Run continously untill something catches fire or an std
	ticker := time.NewTicker(time.Second * 10)

	lastPlaying := ""
	var lastPlayingTime time.Time
	setFallback := false

	for {
		<-ticker.C

		playing, err := check(lfm)
		if err != nil {
			log.Println("Error checking:", err)
			continue
		}

		if playing == lastPlaying {

			if !setFallback && time.Since(lastPlayingTime).Seconds() > float64(flagNoSongDuration) {

				err = s.UpdateListeningStatus(flagNoSong)
				if err != nil {
					log.Println("Error updating status:", err)
				} else {
					log.Println("Updated status to:", flagNoSong)
					setFallback = true
				}

			}
		} else {

			err = s.UpdateListeningStatus(playing)
			if err != nil {
				log.Println("Error updating status:", err)
			} else {
				log.Println("Updated status to:", playing)
				lastPlayingTime = time.Now()
				setFallback = false
				lastPlaying = playing
			}
		}
	}
}

func check(lfm *lastfm.Api) (string, error) {
	recent, err := lfm.User.GetRecentTracks(map[string]interface{}{"user": flagLFMUsername})
	if err != nil {
		return "", err
	}
	if len(recent.Tracks) < 1 {
		return "", errors.New("No tracks")
	}
	track := recent.Tracks[0]
	return censure(track.Name) + " by " + censure(track.Artist.Name), nil
}

func fatal(args ...interface{}) {
	log.Println(args...)
	log.Print("Press enter to quit...")
	var input rune
	fmt.Scanf("%c", &input)
	os.Exit(1)
}
