package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type AnimalSource struct {
	Name string
	URLs []string
}

type Result struct {
	Animals []Animal
}

type Animal struct {
	Name, URL, Note string
	Images          []string
}

func (animal Animal) String() string {
	return fmt.Sprintf("Name: %s URL: %s Note: %s Images: %+v ", animal.Name, animal.URL, animal.Note, animal.Images)
}

var (
	totalAnimalsRegExp, animalRegExp, nameAndURLRegExp, noteRegExp, imagesRegExp *regexp.Regexp

	now     = time.Now()
	dirName = fmt.Sprintf("data-%d%02d%02d%02d%02d%02d", now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())

	dogsSource = &AnimalSource{
		Name: "dogs",
		URLs: []string{
			"http://www.tierschutz-berlin.de/tierheim/tiervermittlung/hunde-sorgenkinder.html",
			"http://www.tierschutz-berlin.de/nc/tierheim/tiervermittlung/hunde.html",
			"http://www.tierschutz-berlin.de/nc/tierheim/tiervermittlung/listenhunde.html",
		},
	}

	catsSource = &AnimalSource{
		Name: "cats",
		URLs: []string{
			"http://www.tierschutz-berlin.de/nc/tierheim/tiervermittlung/katzen-sorgenkinder.html",
			"http://www.tierschutz-berlin.de/nc/tierheim/tiervermittlung/katzen.html",
		},
	}

	animalSources = []*AnimalSource{dogsSource, catsSource}
)

func main() {
	if err := CompileRegExps(); err != nil {
		panic(err)
	}

	if err := os.Mkdir(dirName, 0777); err != nil {
		panic(err)
	}

	sourceNameChannel := make(chan string)
	for _, animalSource := range animalSources {
		go func(source *AnimalSource) {
			parsedAnimals, err := ParseShelterPages(source.URLs)
			if err != nil {
				log.Fatalln(err.Error())
			}

			resultBytes, err := json.Marshal(parsedAnimals)
			if err != nil {
				log.Fatalln(err.Error())
			}

			SaveResult(source.Name, resultBytes)
			sourceNameChannel <- source.Name
		}(animalSource)
	}

	for i := 2; i > 0; i-- {
		select {
		case name := <-sourceNameChannel:
			fmt.Printf("AnimalSource: %s done!\n", name)
		}
	}
}

func CompileRegExps() error {
	var err error
	totalAnimalsRegExp, err = regexp.Compile(`<td>&nbsp;\(([0-9]*) Tiere\)</td>`)
	if err != nil {
		return err
	}

	animalRegExp, err = regexp.Compile(`<table class="item" summary="">(.*?)</tr>\s*</table>\s*</td>\s*</tr>`)
	if err != nil {
		return err
	}

	nameAndURLRegExp, err = regexp.Compile(`<h3><a href="([^"]*)" >([^<]*)</a></h3>`)
	if err != nil {
		return err
	}

	noteRegExp, err = regexp.Compile(`<td><p class="orange">(.*?)</p></td>`)
	if err != nil {
		return err
	}

	imagesRegExp, err = regexp.Compile(`<td class="image"><a [^>]*><img src="(.*?)"[^>]*/>`)
	if err != nil {
		return err
	}

	return nil
}

func ParseShelterPages(urls []string) ([]*Animal, error) {
	animals := []*Animal{}

	for _, url := range urls {
		log.Printf("Parsing %s\n", url)
		animalsFromUrl, err := ParseShelterPage(url)
		if err != nil {
			return animals, err
		}

		animals = append(animals, animalsFromUrl...)
	}

	return animals, nil
}

func ParseShelterPage(url string) ([]*Animal, error) {
	pageContent, err := GetPageContent(url)
	if err != nil {
		return []*Animal{}, err
	}

	totalAnimals, err := GetTotalAnimals(pageContent)
	if err != nil {
		return []*Animal{}, err
	}

	animals := ParseAnimals(pageContent)

	if totalAnimals > 10 {
		animalParsingResultChannel := make(chan []*Animal)

		pagesToParse := (totalAnimals / 10) + 1
		for pointer := 1; pointer < pagesToParse; pointer++ {
			urlWithPointer := fmt.Sprintf("%s?tx_realty_pi1[pointer]=%d", url, pointer)
			log.Printf("Parsing %s\n", urlWithPointer)

			go func(pageUrl string) {
				pageContent, err := GetPageContent(pageUrl)
				if err != nil {
					log.Fatalln(err)
				} else {
					animalParsingResultChannel <- ParseAnimals(pageContent)
				}
			}(urlWithPointer)
		}

		for ; pagesToParse > 1; pagesToParse-- {
			select {
			case parsedAnimals := <-animalParsingResultChannel:
				animals = append(animals, parsedAnimals...)
			}
		}
	}

	return animals, nil
}

func GetPageContent(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return []byte{}, err
	}

	defer resp.Body.Close()
	pageContent, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, err
	}

	pageContentWithoutNewlineEndings := []byte(strings.Replace(string(pageContent), "\n", "", -1))

	SavePageContent(url, pageContentWithoutNewlineEndings)

	return pageContentWithoutNewlineEndings, nil
}

func SavePageContent(url string, pageContentWithoutNewlineEndings []byte) {
	index := strings.LastIndex(url, "/")
	file := fmt.Sprintf("%s%s.html", dirName, url[index:])
	err := ioutil.WriteFile(file, pageContentWithoutNewlineEndings, 0777)
	if err != nil {
		log.Fatalf("Saving pageContentWithoutNewlineEndings to %s failed! Error: %s\n", file, err)
	}
}

func SaveResult(fileName string, result []byte) {
	file := fmt.Sprintf("%s/%s.json", dirName, fileName)
	err := ioutil.WriteFile(file, result, 0777)
	if err != nil {
		log.Fatalf("Saving result to %s failed! Error: %s\n", file, err)
	}
}

func GetTotalAnimals(page []byte) (int, error) {
	totalAnimals := totalAnimalsRegExp.FindSubmatch(page)
	if len(totalAnimals) == 0 {
		//	no pagination -> the page contains max 9 animals
		return 9, nil
	}

	total, err := strconv.ParseInt(string(totalAnimals[1]), 10, 32)
	if err != nil {
		return 0, err
	}

	return int(total), nil
}

func ParseAnimals(page []byte) []*Animal {
	animals := []*Animal{}

	animalsRes := animalRegExp.FindAllSubmatch(page, -1)
	for _, animal := range animalsRes {
		animal, err := ParseAnimal(animal[0])
		if err != nil {
			log.Fatalf("Parsing animal failed! Error: %s\n", err)
			break
		}

		animals = append(animals, animal)
	}

	return animals
}

func ParseAnimal(animalSubmatch []byte) (*Animal, error) {
	url, name, err := ParseNameAndURL(animalSubmatch)
	if err != nil {
		return &Animal{}, err
	}

	note, err := ParseNote(animalSubmatch)
	if err != nil {
		return &Animal{}, err
	}

	images, err := ParseImages(animalSubmatch)
	if err != nil {
		return &Animal{}, err
	}

	return &Animal{Name: name, URL: url, Note: note, Images: images}, nil
}

func ParseNameAndURL(animalSubmatch []byte) (string, string, error) {
	nameAndURLRegExpResult := nameAndURLRegExp.FindAllSubmatch(animalSubmatch, 1)
	if len(nameAndURLRegExpResult) == 1 && len(nameAndURLRegExpResult[0]) == 3 {
		url := "http://www.tierschutz-berlin.de/" + string(nameAndURLRegExpResult[0][1])
		name := string(nameAndURLRegExpResult[0][2])

		return url, name, nil
	}

	return "", "", fmt.Errorf("Parsing nameAndURL failed! nameAndURLRegExpResult: '%+v'\n", nameAndURLRegExpResult)
}

func ParseNote(animalSubmatch []byte) (string, error) {
	noteRegExpResult := noteRegExp.FindAllSubmatch(animalSubmatch, 1)
	if len(noteRegExpResult) != 0 {
		return string(noteRegExpResult[0][1]), nil
	}

	return "", nil
}

func ParseImages(animalSubmatch []byte) ([]string, error) {
	images := []string{}

	imagesRegExpResult := imagesRegExp.FindAllSubmatch(animalSubmatch, -1)
	for i := 0; i < len(imagesRegExpResult); i++ {
		for j := 1; j < len(imagesRegExpResult[i]); j++ {
			imageUrl := "http://www.tierschutz-berlin.de/" + string(imagesRegExpResult[i][j])
			images = append(images, imageUrl)
		}
	}

	return images, nil
}
