package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
	"strconv"
	"strings"
	"cloud.google.com/go/vision/apiv1"
	"golang.org/x/net/context"
	"flag"
	"net/url"
	"github.com/robwil/darebee-workout/nodego"
	"cloud.google.com/go/firestore"
	"net/http"
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const firestoreCollection = "cache"
const firestoreKey = "exercises"
var docNotFoundError = errors.New("document not found")

func detectText(imageURL string) (string, error) {
	ctx := context.Background()
	client, err := vision.NewImageAnnotatorClient(ctx)
	if err != nil {
		return "", err
	}
	image := vision.NewImageFromURI(imageURL)
	annotations, err := client.DetectDocumentText(ctx, image, nil)
	if err != nil {
		return "", err
	}
	return annotations.Text, nil
}

var exceptions = map[string]string{
	"alt-arm-leg-raises": "arm-leg-raises",
	"lunges-exercise":    "forward-lunges",
}

func getVideoName(line string) string {
	line = strings.ToLower(line)
	// extract names only when prefaced with exercise count
	r := regexp.MustCompile(`^(\d+)\s+(.+)`)
	matches := r.FindStringSubmatch(line)
	if len(matches) >= 3 {
		// handle "between sets" instruction; not an exercise so skip it
		if strings.Contains(line, "between") {
			return ""
		}
		// replace non-word chars with hyphen
		r = regexp.MustCompile(`[^\w]`)
		str := r.ReplaceAllString(matches[2], "-")
		// convert any multi hyphen to hyphen (making less sensitive to Google Vision mistakes)
		r = regexp.MustCompile("-+")
		str = r.ReplaceAllString(str, "-")
		// for single word exercises, they append "-exercise" to it
		if !strings.Contains(str, "-") {
			str = str + "-exercise"
		}
		// check for any exceptional cases
		if exceptions[str] != "" {
			return exceptions[str]
		}
		return str
	}
	return ""
}

func getImageURL(workout string, day string) (string, error) {
	workout = strings.ToLower(workout)
	dayNum, err := strconv.Atoi(day)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://darebee.com/images/programs/%s/web/day%02d.jpg", workout, dayNum), nil
}

func getVideoURL(name string) string {
	return fmt.Sprintf("https://darebee.com/exercises/%s.html", name)
}

func getYoutubeEmbed(videoURL string) (string, error) {
	resp, err := http.Get(videoURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	r := regexp.MustCompile(`youtube\.com/embed/([^?]+)\?`)
	matches := r.FindStringSubmatch(string(body))
	if len(matches) >= 2 {
		return matches[1], nil
	}
	return "", nil
}

type exercise struct {
	Name     string
	EmbedURL string
}

type firestoreDoc struct {
	Exercises []exercise `firestore:"exercises,omitempty"`
}

func getFirestoreName(original string) string {
	return strings.NewReplacer("/", "_", ":", "_").Replace(original)
}

func getExercisesFromCache(ctx context.Context, client *firestore.Client, imageURL string) ([]exercise, error) {
	rawDoc, err := client.Collection(firestoreCollection).Doc(getFirestoreName(imageURL)).Get(ctx)
	if err != nil && grpc.Code(err) != codes.NotFound {
		return nil, err
	}
	if !rawDoc.Exists() {
		return nil, docNotFoundError
	}
	doc := &firestoreDoc{}
	if err = rawDoc.DataTo(doc); err != nil {
		return nil, err
	}
	return doc.Exercises, nil
}

func getExercisesForImage(imageURL string) ([]exercise, error) {
	text, err := detectText(imageURL)
	if err != nil {
		return nil, err
	}
	var exercises []exercise
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		videoName := getVideoName(line)
		if videoName == "" {
			continue
		}
		URL := getVideoURL(videoName)
		embedURL, err := getYoutubeEmbed(URL)
		if err != nil {
			return nil, err
		}
		exercises = append(exercises, exercise{Name: line, EmbedURL: embedURL})
	}
	return exercises, nil
}

func saveExercisesForImageToCache(ctx context.Context, client *firestore.Client, imageURL string, exercises []exercise) error {
	docName := getFirestoreName(imageURL)
	doc := &firestoreDoc{Exercises: exercises}
	if _, err := client.Collection(firestoreCollection).Doc(docName).Set(ctx, doc); err != nil {
		return err
	}
	return nil
}

func parseQueryParam(q url.Values, name string) (string, error) {
	raw := q[name]
	if raw == nil {
		return "", fmt.Errorf("param %s not found", name)
	}
	if len(raw) != 1 {
		return "", fmt.Errorf("expected exactly 1 value for param %s, but got %d", name, len(raw))
	}
	return raw[0], nil
}

func printVideos(ctx context.Context, client *firestore.Client) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("GET %s", r.RequestURI)

		// Parse query params
		q := r.URL.Query()
		workout, err := parseQueryParam(q, "workout")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		day, err := parseQueryParam(q, "day")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		// Construct image URL from query params
		imageURL, err := getImageURL(workout, day)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<img src="%s" /><br/>`, imageURL)

		// First try to get exercise from cache
		exercises, err := getExercisesFromCache(ctx, client, imageURL)
		if err != nil && err != docNotFoundError {
			log.Printf("Encountered error when fetching from cache: %v", err)
		}
		// Then fall back to calculating exercises from Google Vision API + HTTP GETs
		if exercises == nil {
			log.Printf("Cache miss, calculating: %s", r.RequestURI)
			exercises, err = getExercisesForImage(imageURL)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			// Put in cache for next time
			err := saveExercisesForImageToCache(ctx, client, imageURL, exercises)
			if err != nil {
				log.Printf("Failed saving exercises for %s to cache: %v", imageURL, err)
			}
		}
		for _, exercise := range exercises {
			if exercise.EmbedURL != "" {
				fmt.Fprintf(w, `
                   <h2>%s</h2>
                   <p>
                       <iframe width="845" height="480" src="//www.youtube.com/embed/%s?rel=0&showinfo=0" frameborder="0" allowfullscreen></iframe>
                   </p>`, exercise.Name, exercise.EmbedURL)
			} else {
				fmt.Fprintf(w, `
                   <h2>%s</h2>
                   <p>Video not found</p>
               `, exercise.Name)
			}
		}
	}
}

func init() {
	nodego.OverrideLogger()
}

func main() {
	flag.Parse()

	// setup Firestore connection
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "darebee-208813")
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	http.HandleFunc(nodego.HTTPTrigger, printVideos(ctx, client))

	nodego.TakeOver()
}
