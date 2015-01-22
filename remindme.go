package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"sync"

	"github.com/asim/quadtree"
)

type user struct {
	id       string
	contacts map[string]bool
	location *quadtree.Point
}

type manager struct {
	sync.RWMutex
	world *quadtree.QuadTree
	users map[string]*user
}

var (
	nearestContacts = 5
	nearestDistance = 10.0 // metres
	defaultManager  = newManager()
)

func newManager() *manager {
	return &manager{
		world: newWorld(),
		users: make(map[string]*user),
	}
}

func newUser(id string) *user {
	return &user{
		id:       id,
		contacts: make(map[string]bool),
	}
}

func newWorld() *quadtree.QuadTree {
	ax := quadtree.NewPoint(0.0, 0.0, nil)
	bx := quadtree.NewPoint(85.0, 185.0, nil)
	bb := quadtree.NewAABB(ax, bx)
	return quadtree.New(bb, 0, nil)
}

func (m *manager) addContacts(id string, contacts []string) {
	m.Lock()
	defer m.Unlock()

	u, ok := m.users[id]
	if !ok {
		log.Printf("new user %s adding contacts", id)
		u = newUser(id)
		m.users[id] = u
	}

	log.Printf("Received contacts %v for user %s", contacts, id)
	for _, contact := range contacts {
		if _, ok := u.contacts[contact]; ok {
			continue
		}
		u.contacts[contact] = true
	}
}

func (m *manager) nearContacts(id string, lat, lon float64) []string {
	m.Lock()
	defer m.Unlock()

	var contacts []string

	u, ok := m.users[id]
	if !ok || len(u.contacts) == 0 {
		return contacts
	}

	c := u.contacts

	// Filter to find users contacts
	filter := func(p *quadtree.Point) bool {
		id, ok := p.Data().(string)
		if !ok {
			return false
		}

		if _, ok := c[id]; !ok {
			return false
		}

		return true
	}

	ax := quadtree.NewPoint(lat, lon, nil) // center
	bx := ax.HalfPoint(nearestDistance)    // top right
	bb := quadtree.NewAABB(ax, bx)

	points := m.world.KNearest(bb, nearestContacts, filter)

	for _, point := range points {
		id, ok := point.Data().(string)
		if !ok {
			continue
		}

		if id == u.id {
			continue
		}

		contacts = append(contacts, id)
	}

	return contacts
}

func (m *manager) updateLocation(id string, lat, lon float64) {
	m.Lock()
	defer m.Unlock()

	u := m.users[id]
	if u == nil {
		log.Printf("new user %s at %f, %f", id, lat, lon)
		u = newUser(id)
		m.users[id] = u
	}

	if u.location == nil {
		u.location = quadtree.NewPoint(lat, lon, id)
		m.world.Insert(u.location)
		return
	}

	x, y := u.location.Coordinates()
	if x == lat && y == lon {
		// no change
		return
	}

	log.Printf("user %s at %f, %f", id, lat, lon)
	location := quadtree.NewPoint(lat, lon, nil)
	m.world.Update(u.location, location)
}

func allHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Bad Request. Non POST", http.StatusBadRequest)
		return
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad Request. Could not read body.", http.StatusBadRequest)
		return
	}

	var data map[string]interface{}
	err = json.Unmarshal(b, &data)
	if err != nil {
		http.Error(w, "Bad Request. Failed to unmarshal request.", http.StatusBadRequest)
		return
	}

	_, ok := data["id"].(string)
	if !ok {
		http.Error(w, "Bad Request. Could not find id.", http.StatusBadRequest)
		return
	}

	distance, ok := data["distance"].(float64)
	if !ok {
		http.Error(w, "Bad Request. Could not find distance.", http.StatusBadRequest)
		return
	}

	numPoints, ok := data["num_points"].(float64)
	if !ok {
		http.Error(w, "Bad Request. Could not find num_points.", http.StatusBadRequest)
		return
	}

	location, ok := data["location"].(map[string]interface{})
	if !ok {
		http.Error(w, "Bad Request. Could not find location.", http.StatusBadRequest)
		return
	}

	lat, ok := location["lat"].(float64)
	if !ok {
		http.Error(w, "Bad Request. Could not parse latitude.", http.StatusBadRequest)
		return
	}

	lon, ok := location["lon"].(float64)
	if !ok {
		http.Error(w, "Bad Request. Could not parse longitude.", http.StatusBadRequest)
		return
	}

	// Filter to find users contacts
	filter := func(p *quadtree.Point) bool {
		return true
	}

	ax := quadtree.NewPoint(lat, lon, nil) // center
	bx := ax.HalfPoint(distance)    // top right
	bb := quadtree.NewAABB(ax, bx)

	points := defaultManager.world.KNearest(bb, int(numPoints), filter)

	users := make(map[string]map[string]float64)

	for _, point := range points {
		id, ok := point.Data().(string)
		if !ok {
			continue
		}

		lat, lon := point.Coordinates()
		users[id] = map[string]float64{"lat": lat, "lon": lon}
	}

	b, err = json.Marshal(users)
	if err != nil {
		http.Error(w, "Internal Server Error. Could not marshal response.", http.StatusInternalServerError)
		return
	}

	_, err = w.Write(b)
	if err != nil {
		http.Error(w, "Internal Server Error. Could not write response.", http.StatusInternalServerError)
		return
	}
}

func contactHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Bad Request. Non POST", http.StatusBadRequest)
		return
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad Request. Could not read body.", http.StatusBadRequest)
		return
	}

	var data map[string]interface{}
	err = json.Unmarshal(b, &data)
	if err != nil {
		http.Error(w, "Bad Request. Failed to unmarshal request.", http.StatusBadRequest)
		return
	}

	id, ok := data["id"].(string)
	if !ok {
		http.Error(w, "Bad Request. Could not find id.", http.StatusBadRequest)
		return
	}

	icontacts, ok := data["contacts"].([]interface{})
	if !ok {
		http.Error(w, "Bad Request. Could not find contacts.", http.StatusBadRequest)
		return
	}

	var contacts []string

	for _, contact := range icontacts {
		c, ok := contact.(string)
		if !ok {
			http.Error(w, "Bad Request. Failed to parse contacts.", http.StatusBadRequest)
			return
		}

		contacts = append(contacts, c)
	}

	defaultManager.addContacts(id, contacts)
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Bad Request. Non POST", http.StatusBadRequest)
		return
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad Request. Could not read body.", http.StatusBadRequest)
		return
	}

	var data map[string]interface{}
	err = json.Unmarshal(b, &data)
	if err != nil {
		http.Error(w, "Bad Request. Failed to unmarshal request.", http.StatusBadRequest)
		return
	}

	id, ok := data["id"].(string)
	if !ok {
		http.Error(w, "Bad Request. Could not find id.", http.StatusBadRequest)
		return
	}

	location, ok := data["location"].(map[string]interface{})
	if !ok {
		http.Error(w, "Bad Request. Could not find location.", http.StatusBadRequest)
		return
	}

	lat, ok := location["lat"].(float64)
	if !ok {
		http.Error(w, "Bad Request. Could not parse latitude.", http.StatusBadRequest)
		return
	}

	lon, ok := location["lon"].(float64)
	if !ok {
		http.Error(w, "Bad Request. Could not parse longitude.", http.StatusBadRequest)
		return
	}

	defaultManager.updateLocation(id, lat, lon)
}

func nearHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Bad Request. Non POST", http.StatusBadRequest)
		return
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad Request. Could not read body.", http.StatusBadRequest)
		return
	}

	var data map[string]interface{}
	err = json.Unmarshal(b, &data)
	if err != nil {
		http.Error(w, "Bad Request. Failed to unmarshal request.", http.StatusBadRequest)
		return
	}

	id, ok := data["id"].(string)
	if !ok {
		http.Error(w, "Bad Request. Could not find id.", http.StatusBadRequest)
		return
	}

	location, ok := data["location"].(map[string]interface{})
	if !ok {
		http.Error(w, "Bad Request. Could not find location.", http.StatusBadRequest)
		return
	}

	lat, ok := location["lat"].(float64)
	if !ok {
		http.Error(w, "Bad Request. Could not parse latitude.", http.StatusBadRequest)
		return
	}

	lon, ok := location["lon"].(float64)
	if !ok {
		http.Error(w, "Bad Request. Could not parse longitude.", http.StatusBadRequest)
		return
	}

	contacts := defaultManager.nearContacts(id, lat, lon)

	response := map[string]interface{}{
		"contacts": contacts,
	}

	b, err = json.Marshal(response)
	if err != nil {
		http.Error(w, "Internal Server Error. Could not marshal contacts.", http.StatusInternalServerError)
		return
	}

	_, err = w.Write(b)
	if err != nil {
		http.Error(w, "Internal Server Error. Could not write response.", http.StatusInternalServerError)
		return
	}
}

func main() {
	// Add Contacts
	http.HandleFunc("/contacts", contactHandler)

	// Update Location
	http.HandleFunc("/ping", pingHandler)

	// Find Nearby Contacts
	http.HandleFunc("/near", nearHandler)

	// Find Nearby Contacts
	http.HandleFunc("/_all", allHandler)

	err := http.ListenAndServe(":9999", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

/*
	POST /contacts -- add contact to a users contact list
	request: {id: user_id, contacts: [ contact1, contact2, ... ]}

	POST /ping -- update user location
	request: {id: user_id, location: {lat: lat, lon: lon, alt: altitude}}

	POST /near -- get nearby contacts
	request: {id: user_id, location: {lat: lat, lon: lon, alt: altitude}}
	response: [ contact1, contact2, ... ]
*/

/*
func test() {
	b, _ := ioutil.ReadFile("tube.csv")
	lines := strings.Split(string(b), "\n")

	points := make(map[string]*quadtree.Point)

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		data := strings.Split(line, ",")
		x, _ := strconv.ParseFloat(data[1], 64)
		y, _ := strconv.ParseFloat(data[2], 64)

		points[data[0]] = quadtree.NewPoint(x, y, nil)
	}

	for _, p := range points {
		q.Insert(p)
	}

	bb = quadtree.NewAABB(
		quadtree.NewPoint(51.50, -0.12, nil),
		quadtree.NewPoint(0.05, 0.05, nil),
	)

	res := q.Search(bb)
	res2 := q.KNearest(bb, 5)

	for _, p := range res {
		fmt.Println(p)
	}

	fmt.Println("")

	for _, p := range res2 {
		fmt.Println(p)
	}

	bb = quadtree.NewAABB(
		quadtree.NewPoint(51.52, -0.13, nil),
		quadtree.NewPoint(0.02, 0.02, nil),
	)

	fmt.Println("")

	res2 = q.KNearest(bb, 5)

	for _, p := range res2 {
		fmt.Println(p)
	}
}
*/

