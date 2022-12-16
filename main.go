package main

import (
	"bufio"
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"os"
	"strconv"
	"strings"
	"sync"
)

type Entity struct {
	uri             string
	lcLabel         string
	prefLabel       string
	definition      string
	lcSynonyms      []string
	synonyms        []string
	instances       []string
	fromScore       int
	toScore         int
	refScore        int
	annotationScore float64
	entityType      string
}

var prefLabelRT = "<http://www.w3.org/2004/02/skos/core#prefLabel>"
var definitionRT = "<http://www.w3.org/2004/02/skos/core#definition>"
var evidenceRT = "<http://schema.org/evidenceLevel>"
var typeRT = "<http://www.w3.org/1999/02/22-rdf-syntax-ns#type>"

// altLabelRT := "http://www.w3.org/2004/02/skos/core#altLabel"
var classURI = "<http://www.w3.org/2002/07/owl#Class>"

func main() {

	fmt.Print("Hello, Go!\n")
	// f, err := os.Open("test-prot.nt")

	mongoURI := "mongodb://localhost:27027"
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoURI))
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()
	parseGeneProt("9606", "prot", client)
	parseGeneProt("9606", "gene", client)
	fmt.Printf("Done!")
}

func parseGeneProt(taxon string, graph string, client *mongo.Client) {
	f, err := os.Open("rdf/" + graph + "/" + taxon + ".nt")
	if err != nil {
		fmt.Print("Error opening file: ", err)
	}
	defer f.Close()
	// protDB := client.Database("metadb").Collection("prot")
	lineNumber := 0
	scanner := bufio.NewScanner(f)
	protMap := make(map[string]Entity)

	for scanner.Scan() {
		line := scanner.Text()
		lineNumber++
		components := strings.SplitN(line, " ", 3)
		sub := components[0]
		predicate := components[1]
		value := strings.Replace(strings.Replace(components[2], "\"", "", 2), " .", "", 1)

		uri := removeLTGT(sub)
		if !strings.HasPrefix(uri, "http://rdf.biogateway.eu/"+graph) {
			continue
		}
		if predicate == prefLabelRT {
			if entry, ok := protMap[uri]; ok {
				entry.prefLabel = value
				entry.lcLabel = strings.ToLower(value)
				protMap[uri] = entry
			} else {
				protMap[uri] = Entity{
					uri:       uri,
					prefLabel: value,
					lcLabel:   strings.ToLower(value),
				}
			}
		} else if predicate == definitionRT {
			if entry, ok := protMap[uri]; ok {
				entry.definition = value
				protMap[uri] = entry
			} else {
				protMap[uri] = Entity{
					uri:        uri,
					definition: value}
			}
		} else if predicate == evidenceRT {
			if entry, ok := protMap[uri]; ok {
				if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
					entry.annotationScore = floatValue
					protMap[uri] = entry
				}

			} else {
				protMap[uri] = Entity{
					uri:        uri,
					definition: value}
			}
		} else if predicate == typeRT {
			if value == classURI {
				if entry, ok := protMap[uri]; ok {
					entry.entityType = removeLTGT(value)
				} else {
					protMap[uri] = Entity{
						uri:        uri,
						entityType: removeLTGT(value)}
				}
			} else {
				// This is an instance of another class.
				// TODO: Add it to the array of instances for the root class.
			}
		}
		if lineNumber%1000 == 0 {
			fmt.Println("Parsed line number", lineNumber)
		}
	}
	threadCount := 10
	entitiesPerThread := (len(protMap) / threadCount) + 1
	entities := make([][]Entity, threadCount)

	for index, _ := range entities {
		entities[index] = make([]Entity, entitiesPerThread)
		i := 0
		for key, prot := range protMap {
			if i > entitiesPerThread-1 {
				continue
			}
			entities[index][i] = prot
			delete(protMap, key)
			i++
		}
	}

	var waitGroup sync.WaitGroup
	waitGroup.Add(threadCount)

	for index, list := range entities {
		go func(i int, list []Entity) {
			defer waitGroup.Done()
			insertEntitiesToDB(list, client, i, graph)
		}(index, list)
	}
	waitGroup.Wait()
}

func removeLTGT(value string) string {
	return strings.Replace(strings.Replace(value, ">", "", 1), "<", "", 1)
}

func insertEntitiesToDB(entities []Entity, client *mongo.Client, index int, graph string) {
	updateOptions := options.Update().SetUpsert(true)
	protDB := client.Database("metadb").Collection(graph)
	protNumber := 0
	for _, prot := range entities {
		protNumber++
		_, err := protDB.UpdateOne(
			context.TODO(),
			bson.D{{"uri", prot.uri}},
			bson.D{{"$set", bson.D{
				{"uri", prot.uri},
				{"prefLabel", prot.prefLabel},
				{"lcLabel", prot.lcLabel},
				{"definition", prot.definition},
				{"annotationScore", prot.annotationScore},
			}}}, updateOptions)
		if err != nil {
			panic(err)
		}
		if protNumber%1000 == 0 {
			fmt.Println(graph, "Thread", index, "Inserted", protNumber, "into mongoDB")
		}
	}
}

func generateEntityQuery(graph string, constraint string) string {
	query := `SELECT DISTINCT ?uri ?prefLabel ?definition
	WHERE {
		GRAPH <http://rdf.biogateway.eu/graph/%s> {
		?uri skos:prefLabel|rdfs:label ?prefLabel .
		%s
		OPTIONAL { ?uri skos:definition ?definition . }
	}}`
	return fmt.Sprintf(query, graph, constraint)
}
