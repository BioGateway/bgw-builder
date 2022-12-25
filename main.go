package main

import (
	"bufio"
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
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
	pubMeds         []string
}

var prefLabelRT = "<http://www.w3.org/2004/02/skos/core#prefLabel>"
var definitionRT = "<http://www.w3.org/2004/02/skos/core#definition>"
var synonymRT = "<http://www.w3.org/2004/02/skos/core#altLabel>"
var instanceRT = "<http://schema.org/evidenceOrigin>"
var evidenceRT = "<http://schema.org/evidenceLevel>"
var taxonRT = "<http://purl.obolibrary.org/obo/RO_0000052>"
var typeRT = "<http://www.w3.org/1999/02/22-rdf-syntax-ns#type>"
var pubMedRT = "<http://semanticscience.org/resource/SIO_000772>"

var taxonPrefix = "http://purl.obolibrary.org/obo/NCBITaxon_"

var threadCount = 10

// altLabelRT := "http://www.w3.org/2004/02/skos/core#altLabel"
var classURI = "<http://www.w3.org/2002/07/owl#Class>"

/*
var taxa = []string{
	"3055",
	"3702",
	"4577",
	"6239",
	"7227",
	"7955",
	"9031",
	"9606",
	"9615",
	"9823",
	"9913",
	"9986",
	"10090",
	"10116",
	"36329",
	"39947",
	"44689",
	"284812",
	"367110",
	"559292",
}
*/

var taxa = []string{
	"9606",
}

func main() {

	fmt.Print("MetaDB Generator started...\n")

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

	for _, taxon := range taxa {
		parseGOFile("go/go-basic.obo", client)
		fmt.Println("Parsing RDFs for taxon", taxon)
		// parseEntityRDF(taxon, "prot", client)
		//parseEntityRDF(taxon, "gene", client)
	}

	fmt.Printf("Done!")
}

func parseEntityRDF(taxon string, graph string, client *mongo.Client) {
	f, err := os.Open("rdf/" + graph + "/" + taxon + ".nt")
	if err != nil {
		fmt.Print("Error opening file: ", err)
	}
	defer f.Close()
	// protDB := client.Database("metadb").Collection("prot")
	lineNumber := 0
	scanner := bufio.NewScanner(f)
	entityMap := make(map[string]Entity)

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
			if entry, ok := entityMap[uri]; ok {
				entry.prefLabel = value
				entry.lcLabel = strings.ToLower(value)
				entityMap[uri] = entry
			} else {
				entityMap[uri] = Entity{
					uri:       uri,
					prefLabel: value,
					lcLabel:   strings.ToLower(value),
				}
			}
		} else if predicate == definitionRT {
			if entry, ok := entityMap[uri]; ok {
				entry.definition = value
				entityMap[uri] = entry
			} else {
				entityMap[uri] = Entity{
					uri:        uri,
					definition: value}
			}
		} else if predicate == synonymRT {
			if entry, ok := entityMap[uri]; ok {
				if len(entry.synonyms) > 0 {
					entry.synonyms = append(entry.synonyms, value)
				} else {
					entry.synonyms = []string{value}
				}
				entityMap[uri] = entry
			} else {
				entityMap[uri] = Entity{
					uri:      uri,
					synonyms: []string{value},
				}
			}
		} else if predicate == instanceRT {
			instanceURI := removeLTGT(value)
			if entry, ok := entityMap[uri]; ok {
				if len(entry.instances) > 0 {
					entry.instances = append(entry.instances, instanceURI)
				} else {
					entry.instances = []string{instanceURI}
				}
				entityMap[uri] = entry
			} else {
				entityMap[uri] = Entity{
					uri:       uri,
					instances: []string{instanceURI},
				}
			}
		} else if predicate == pubMedRT {
			pubMedURI := removeLTGT(value)
			if entry, ok := entityMap[uri]; ok {
				if len(entry.pubMeds) > 0 {
					entry.pubMeds = append(entry.pubMeds, pubMedURI)
				} else {
					entry.pubMeds = []string{pubMedURI}
				}
				entityMap[uri] = entry
			} else {
				entityMap[uri] = Entity{
					uri:     uri,
					pubMeds: []string{pubMedURI},
				}
			}
		} else if predicate == evidenceRT {
			if entry, ok := entityMap[uri]; ok {
				if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
					entry.annotationScore = floatValue
					entityMap[uri] = entry
				}

			} else {
				entityMap[uri] = Entity{
					uri:        uri,
					definition: value}
			}
		} else if predicate == typeRT {
			if value == classURI {
				if entry, ok := entityMap[uri]; ok {
					entry.entityType = removeLTGT(value)
				} else {
					entityMap[uri] = Entity{
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
	entitiesPerThread := (len(entityMap) / threadCount) + 1
	entities := make([][]Entity, threadCount)

	for index, _ := range entities {
		entities[index] = make([]Entity, entitiesPerThread)
		i := 0
		for key, prot := range entityMap {
			if i > entitiesPerThread-1 {
				continue
			}
			entities[index][i] = prot
			delete(entityMap, key)
			i++
		}
	}

	var waitGroup sync.WaitGroup
	waitGroup.Add(threadCount)

	for index, list := range entities {
		go func(i int, list []Entity) {
			defer waitGroup.Done()
			insertEntitiesToDB(list, client, i, graph, taxon)
		}(index, list)
	}
	waitGroup.Wait()
}

func parseGOFile(filePath string, client *mongo.Client) {
	graph := "goall"
	f, err := os.Open("rdf/" + filePath)
	if err != nil {
		fmt.Print("Error opening file: ", err)
	}
	defer f.Close()
	// protDB := client.Database("metadb").Collection("prot")
	lineNumber := 0
	scanner := bufio.NewScanner(f)
	entityMap := make(map[string]Entity)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "[Term]" {
			scanner.Scan()
			idLine := scanner.Text()
			scanner.Scan()
			nameLine := scanner.Text()
			scanner.Scan()
			namespaceLine := scanner.Text()
			id := strings.Replace(idLine, "id: ", "", 1)
			name := strings.Replace(nameLine, "name: ", "", 1)
			namespace := strings.Replace(namespaceLine, "namespace: ", "", 1)
			uri := "http://purl.obolibrary.org/obo/" + strings.Replace(id, ":", "_", 1)
			entityMap[uri] = Entity{
				uri:       uri,
				prefLabel: name,
				lcLabel:   strings.ToLower(name),
				// Replacing the underscore with space, and capitalizing each word.
				definition: cases.Title(language.English, cases.Compact).String(strings.Replace(namespace, "_", " ", 1)),
			}
		}
		lineNumber++
	}

	entitiesPerThread := (len(entityMap) / threadCount) + 1
	entities := make([][]Entity, threadCount)

	for index, _ := range entities {
		entities[index] = make([]Entity, entitiesPerThread)
		i := 0
		for key, prot := range entityMap {
			if i > entitiesPerThread-1 {
				continue
			}
			entities[index][i] = prot
			delete(entityMap, key)
			i++
		}
	}

	var waitGroup sync.WaitGroup
	waitGroup.Add(threadCount)

	for index, list := range entities {
		go func(i int, list []Entity) {
			defer waitGroup.Done()
			insertSimpleEntitiesToDB(list, client, i, graph)
		}(index, list)
	}
	waitGroup.Wait()

}

func removeLTGT(value string) string {
	return strings.Replace(strings.Replace(value, ">", "", 1), "<", "", 1)
}

/*
 */

func insertSimpleEntitiesToDB(entities []Entity, client *mongo.Client, index int, graph string) {
	updateOptions := options.Update().SetUpsert(true)
	// insertOptions := options.InsertOne().SetBypassDocumentValidation(true)
	entityDB := client.Database("metadb").Collection(graph)
	dbIndices := []mongo.IndexModel{
		{Keys: bson.M{"uri": 1}},
		{Keys: bson.M{"lcLabel": 1}},
		{Keys: bson.M{"lcSynonyms": 1}},
		{Keys: bson.M{"refScore": 1}},
		{Keys: bson.M{"definition": "text"}},
	}

	_, err := entityDB.Indexes().CreateMany(context.TODO(), dbIndices)
	if err != nil {
		panic(err)
	}
	entityNumber := 0
	timestamp := time.Now().Unix()
	for _, entity := range entities {
		entityNumber++
		lcSynonyms := []string{}
		for _, v := range entity.synonyms {
			lcSynonyms = append(lcSynonyms, strings.ToLower(v))
		}
		doc := bson.M{
			"uri":             entity.uri,
			"prefLabel":       entity.prefLabel,
			"lcLabel":         entity.lcLabel,
			"definition":      entity.definition,
			"annotationScore": entity.annotationScore,
			"synonyms":        entity.synonyms,
			"lcSynonyms":      lcSynonyms,
			"instances":       entity.instances,
			// "pubMedRefs":      entity.pubMeds,
			"refScore": len(entity.pubMeds),
		}
		_, err := entityDB.UpdateOne(
			context.TODO(),
			bson.M{"uri": entity.uri},
			bson.M{"$set": doc},
			updateOptions)
		if err != nil {
			panic(err)
		}
		if entityNumber%1000 == 0 {
			nowTime := time.Now().Unix()
			duration := nowTime - timestamp
			timestamp = nowTime
			fmt.Println("Thread", index, "Inserted", entityNumber, "into mongoDB graph", graph, "in", duration, "seconds")
		}
	}
}

func insertEntitiesToDB(entities []Entity, client *mongo.Client, index int, graph string, taxon string) {
	updateOptions := options.Update().SetUpsert(true)
	// insertOptions := options.InsertOne().SetBypassDocumentValidation(true)
	entityDB := client.Database("metadb").Collection(graph)
	dbIndices := []mongo.IndexModel{
		{Keys: bson.M{"uri": 1}},
		{Keys: bson.M{"lcLabel": 1}},
		{Keys: bson.M{"lcSynonyms": 1}},
		{Keys: bson.M{"refScore": 1}},
		{Keys: bson.M{"taxon": 1}},
		{Keys: bson.M{"definition": "text"}},
	}

	_, err := entityDB.Indexes().CreateMany(context.TODO(), dbIndices)
	if err != nil {
		panic(err)
	}
	entityNumber := 0
	timestamp := time.Now().Unix()
	for _, entity := range entities {
		entityNumber++
		lcSynonyms := []string{}
		for _, v := range entity.synonyms {
			lcSynonyms = append(lcSynonyms, strings.ToLower(v))
		}
		doc := bson.M{
			"uri":             entity.uri,
			"prefLabel":       entity.prefLabel,
			"lcLabel":         entity.lcLabel,
			"definition":      entity.definition,
			"annotationScore": entity.annotationScore,
			"synonyms":        entity.synonyms,
			"lcSynonyms":      lcSynonyms,
			"instances":       entity.instances,
			"taxon":           taxonPrefix + taxon,
			// "pubMedRefs":      entity.pubMeds,
			"refScore": len(entity.pubMeds),
		}
		_, err := entityDB.UpdateOne(
			context.TODO(),
			bson.M{"uri": entity.uri},
			bson.M{"$set": doc},
			updateOptions)
		if err != nil {
			panic(err)
		}
		if entityNumber%1000 == 0 {
			nowTime := time.Now().Unix()
			duration := nowTime - timestamp
			timestamp = nowTime
			fmt.Println(taxon, "Thread", index, "Inserted", entityNumber, "into mongoDB graph", graph, "in", duration, "seconds")
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
