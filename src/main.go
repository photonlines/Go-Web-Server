// Simple golang webserver with logging, tracing, health check, graceful shutdown, as well
// as demo applications

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	REQUEST_ID_KEY         = 8888
	READ_TIMEOUT           = 10
	WRITE_TIMEOUT          = 10
	IDLE_TIMEOUT           = 30
	LOG_FILE_NAME          = "server_log.log"
	DEFAULT_SERVER_ADDRESS = "8888"
)

var (
	listenAddr string
	healthy    int32
)

func main() {

	// Implement command line flag parsing, allowing the user to enter the http service address
	// which defaults to 8888 (i.e. http://localhost:8888/)
	flag.StringVar(&listenAddr, "address", ":"+DEFAULT_SERVER_ADDRESS, "http service address")
	flag.Parse()

	// Prepare our log file for writing / appending new logging info:
	logFile, err := os.OpenFile(LOG_FILE_NAME, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)

	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	// Ensure that our log file is closed when we're done serving
	defer logFile.Close()

	// We log the results to our file with the date and time in the local timezone included
	// or prefixed to each entry.
	logger := log.New(logFile, "http: ", log.LstdFlags)

	// Create a new request ID based on the number of nanoseconds elapsed from January 1, 1970 UTC
	// until today / now.
	nextRequestID := func() string {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	// Create the custom HTTP server with the parameters we want to use along with our logging,
	// tracing and route handlers
	server := &http.Server{
		Addr:         listenAddr,
		Handler:      tracingHandler(nextRequestID)(loggingHandler(logger)(routeHandler())),
		ErrorLog:     logger,
		ReadTimeout:  READ_TIMEOUT * time.Second,
		WriteTimeout: WRITE_TIMEOUT * time.Second,
		IdleTimeout:  IDLE_TIMEOUT * time.Second,
	}

	// Go signal notification works by sending os.Signal values on a channel. We’ll create a
	// channel to receive these notifications (we’ll also make one to notify us when the
	// program can exit).
	doneChannel := make(chan bool)
	quitChannel := make(chan os.Signal, 1)

	// signal.Notify registers the quit channel to receive notifications of the specified
	// signals. In our case below, we register our quit channel to receive OS interrupt (same
	// as CTRL + C) or SIGTERM (kill / terminate) signals so that we can handle shut downs
	// gracefully
	signal.Notify(quitChannel, os.Interrupt, syscall.SIGTERM)

	// Create and execute a function which handles unexpected interrupts / shutdowns:
	go func() {
		// Trigger when our quit channel receives a signal
		<-quitChannel

		logger.Println("Server is shutting down...")

		// Atomically update our health state indicator to 'not-healthy'
		atomic.StoreInt32(&healthy, 0)

		// Create an empty context and set the deadline to 30 seconds
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Disable HTTP keep-alives
		server.SetKeepAlivesEnabled(false)

		// Gracefully shut down the server without interrupting any active connections.The
		// shutdown function works by first closing all open listeners, then closing all idle
		// connections, and then waiting indefinitely for connections to return to an idle
		// state. Afterwards, it can be shut down.
		if err := server.Shutdown(ctx); err != nil {
			// If we encounter an issue with our shutdown, we log it along with the error
			logger.Fatalf("Could not gracefully shutdown the server: %v\n", err)
		}

		close(doneChannel)

	}()

	logger.Println("Server is ready to handle requests at ", listenAddr)

	// Atomically update our health state indicator to 'healthy'
	atomic.StoreInt32(&healthy, 1)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Could not listen on %s: %v\n", listenAddr, err)
	}

	// If we receive a signal via the done channel, we log the event:
	<-doneChannel
	logger.Println("Server stopped")

}

// This is our route handler:
func routeHandler() *http.ServeMux {

	// Create a new multiplexer / router to route our requests to the correct handler
	router := http.NewServeMux()

	// Main web application handlers:
	router.HandleFunc("/", indexHandler)
	router.HandleFunc("/excel", excelHandler)
	router.HandleFunc("/qr-code-generator", qrCodeHandler)
	router.HandleFunc("/svg", svgHandler)
	router.HandleFunc("/sphere", sphereHandler)

	// Health and logging handlers for demoing extra functionality
	router.HandleFunc("/health", healthHandler)
	router.HandleFunc("/log", logHandler)

	return router

}

// HTML data element which is used to pass in the required data we want to include in our
// applications / html templates.
type HtmlData struct {
	Title       string
	Description string
	Keywords    string
	Author      string
	CssFiles    []string
	JsFiles     []string
	CssScript   template.HTML
	JsScript    template.HTML
	BodyContent template.HTML
}

// This is our main CSS script. Currently, we pass this into our template each time we
// construct one. Ideally, this should be a nested template or file which is included
// as part of our main template. The only reason the raw data is included here is to
// make the code more readable. You can find the raw CSS file (called style.css) in the
// css folder.
const MAIN_CSS_TEMPLATE = `
<style>

	/* Horizontal NavBar */

	nav a {
		text-decoration: none;
		color: #fff;
		font-size: 110%;
		font-family: 'Open Sans', sans-serif;   
	}

	li {
		text-decoration: none;
		display: inline-block;
		margin: 8% 4% -1% 4%;
		padding: 1%;
	}

	/* Adding NavBar Background */

	.main-nav {
		background: #000000;
		text-align: center;
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		opacity: 0.6;
		z-index: 9999;
		margin: -10%;
	}

	/* Setting Hover States */

	a:hover {
		color: #a9a9a9;
	}

	a:active {
		color: #a9a9a9;
	}

	/* Body Styles */

	body {
		margin: 0;
		font-family: 'Open Sans', sans-serif; 
		font-weight: 100;
	}

	body, html
	{
		height: 100%;
	}

	#table-container
	{
		display:    table;
		text-align: center;
		width:      100%;
		height:     100%;
	}

	#container
	{
		display:        table-cell;
		vertical-align: middle;
	}

	#main
	{
		display: inline-block;
	}

	#spreadsheet
	{
		margin: 20px;
	}

	.main-content {

		position: absolute;
		left: 50%;
		top: 50%;
		transform: translate(-50%, -50%);
		
		width: 70%;
		height: 60%;

		padding-top: 40px;  
		padding-bottom: 20px;  
		padding-left: 20px;  
		padding-right: 20px;  

		color: black;
		text-align: center;

	}

	/* Form elements for inputting / submitting QR Codes */

	form input {
		float:center;
		clear:both;
	}
	
	form input {
		margin:15px 0;
		padding:15px 10px;
		width:40%;
		text-align: center;
		outline:none;
		border:1px solid #bbb;
		border-radius:20px;
		display:inline-block;
		-webkit-box-sizing:border-box;
		   -moz-box-sizing:border-box;
				box-sizing:border-box;
		-webkit-transition:0.2s ease all;
		   -moz-transition:0.2s ease all;
			-ms-transition:0.2s ease all;
			 -o-transition:0.2s ease all;
				transition:0.2s ease all;
	}
	
	form input[type=text]:focus {
		border-color:cornflowerblue;
	}

</style>
`

// This is our main HTML template which is used to construct our web applications. Ideally, this
// should be read in from a template file stored in our templates folder, but we include the full
// string here for readability purposes. You can find the template file in the templates folder -
// it's called main.tmpl.
const MAIN_HTML_TEMPLATE = `
<!DOCTYPE html>
<html lang="en">

<head>
	<meta charset="utf-8">
	<meta name="description" content="{{ .Description }}">
	<meta name="keywords" content="{{ .Keywords }}">
	<meta name="author" content="{{ .Author }}">

	<title>{{ .Title }}</title>

	{{ range $index, $cssFileLocation := .CssFiles }}
	<link rel="stylesheet" type="text/css" href="{{ $cssFileLocation }}">
	{{ end }}

	{{ range $index, $jsFileLocation := .JsFiles }}
	<script src="{{ $jsFileLocation }}"></script>
	{{ end }}

	{{ .CssScript }}
	
</head>

<header>
    <div class="main-nav">
        <nav>
			<ul>
				<li><a href="/"/>Home</a></li>
				<li><a href="/excel"/>Excel App</a></li>
				<li><a href="/qr-code-generator"/>QR Code Generator</a></li>
				<li><a href="/svg">SVG Example</a></li>
				<li><a href="/sphere"/>Sphere</a></li>
			</ul>
        </nav>
    </div>
</header>

<body>
	{{ .BodyContent }}
</body>

{{ .JsScript }}

</html> 
`

// Our main index handler. This page displays basic intro text with a description of basic
// functionality and the libraries we use to construct our demo applications.
func indexHandler(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	// Let's create the HTML data we want to pass to our template
	htmlData := HtmlData{
		Title:       "Golang Web Server",
		Description: "This is a simple golang webserver example with built in logging, tracing, a health check, and graceful shutdown.",
		Keywords:    "golang web server",
		Author:      "",
		CssFiles: []string{
			"https://fonts.googleapis.com/css?family=Open+Sans",
		},
		CssScript: template.HTML(MAIN_CSS_TEMPLATE),
		BodyContent: template.HTML(
			`<div class = "main-content">
			 	<h2>Simple Golang Web Server</h2>
				<p>This is a simple golang web server example with built in logging, tracing, a health check, and graceful shutdown.</p>
				<br>
				<h4>It also includes a few demo web applications, including:</h4>
				<p>An Excel / Spreadsheet application using <a href="https://bossanova.uk/jexcel/v2/">JExcel</a></p>
				<p>A QR Code Generator using <a href="https://developers.google.com/chart">Google Charts API</a></p>
				<p>An SVG drawing example (taken from <a href="https://github.com/adonovan/gopl.io/blob/master/ch3/surface/main.go">The Go Programming Language</a>)</p>
				<p>A 3D sphere example using <a href="https://threejs.org/">THREE.JS</a><p>
			</div>
		`),
	}

	// Create a new template using our main HTML string
	indexTemplate, err := template.New("index").Parse(MAIN_HTML_TEMPLATE)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Execute the template / tpl passing in our HTML data elements and writing the results
	// to our response writer
	if err := indexTemplate.Execute(w, htmlData); err != nil {
		fmt.Println(err)
	}
}

// This is our handler for demoing simple excel editing functionality using JExcel. The source
// for this functionality can be found here: https://github.com/paulhodel/jexcel
func excelHandler(w http.ResponseWriter, r *http.Request) {

	// Data we pass into our template to construct our application / HTML page
	htmlData := HtmlData{
		Title:       "Golang Excel Web Editor",
		Description: "Simple golang webserver example with JExcel.",
		Keywords:    "golang web server jexcel spreadsheet",
		Author:      "",
		CssFiles: []string{
			"https://cdnjs.cloudflare.com/ajax/libs/jexcel/3.5.0/jexcel.min.css",
			"https://bossanova.uk/jsuites/v2/jsuites.css",
			"https://fonts.googleapis.com/css?family=Open+Sans",
		},
		JsFiles: []string{
			"https://cdnjs.cloudflare.com/ajax/libs/jquery/3.4.1/jquery.min.js",
			"https://cdnjs.cloudflare.com/ajax/libs/jexcel/3.5.0/jexcel.min.js",
			"https://bossanova.uk/jsuites/v2/jsuites.js",
		},
		CssScript: template.HTML(MAIN_CSS_TEMPLATE),
		BodyContent: template.HTML(`
		<div id="table-container">
			<div id="container">
				<div id="main">
					<h2>Simple Excel Editor</h2>	
					<div id="spreadsheet"></div>				
					<script>
						
						// The number of columns, rows to include 
						var options = {
							minDimensions:[20,15],
						}		

						$('#spreadsheet').jexcel(options); 	

					</script>
				</div>
			</div>
		</div>
		`),
	}

	// Create a new template using our main HTML string
	excelTemplate, err := template.New("excel").Parse(MAIN_HTML_TEMPLATE)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Execute the template / tpl passing in our HTML data elements and writing the results
	// to our response writer
	if err := excelTemplate.Execute(w, htmlData); err != nil {
		fmt.Println(err)
	}

}

// This is the handler used for constructing our QR Code generator. The generator prompts
// the user to enter a QR code and uses the Google Chart API to fetch the QR code
func qrCodeHandler(w http.ResponseWriter, r *http.Request) {

	// This is a template string we use to construct our body content. We check to see if we have a
	// defined QR code, and if so, we use the Google API for fetching the QR code image. If no
	// QR code is input, we don't display anything. You can find the raw template file in the
	// templates sub-directory titled qr.code.body.tmpl.
	var bodyHtmlTemplate = `
	 <div class = "main-content">
		<h2>QR Code Generator</h2>	
		<form action="/qr-code-generator" name="qr_code_form" method="GET">
			<input maxLength=512 size=80 name="qr_code_text" value="" title="Text to QR Encode">
			<br>
			<input type=submit value="Show QR" name="qr_code_submission">
			<br>
			{{if .QRCode}}
			<img src="http://chart.apis.google.com/chart?chs=300x300&cht=qr&choe=UTF-8&chl={{.QRCode}}" />
			<br>
			{{.QRCode}}
			<br>
			<br>
			{{end}}				
		</form>
	</div>
	`

	// Check to see if we have a QR code specified in our request
	qrCode := r.URL.Query().Get("qr_code_text")

	// Construct the data element which we will use to pass in the QR code to our template
	data := struct {
		QRCode string
	}{
		QRCode: qrCode,
	}

	// Create a new template / tpl for our body template
	bodyTemplate, err := template.New("qr.code.generator.body").Parse(bodyHtmlTemplate)

	// Since we don't want to pass in our HTML to our response writer quite yet, we store
	// the template file results in memory via a bytes buffer
	var tpl bytes.Buffer

	if err := bodyTemplate.Execute(&tpl, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert our encoded template data to a string which we will use to pass on to our
	// main template
	bodyHTML := tpl.String()

	// Let's create the data we'll use to pass to our main HTML template
	htmlData := HtmlData{
		Title:       "Golang QR Code Generator",
		Description: "Simple Golang QR code generator using Google API.",
		Keywords:    "golang web server qr code generator google api",
		Author:      "",
		CssScript:   template.HTML(MAIN_CSS_TEMPLATE),
		BodyContent: template.HTML(bodyHTML),
	}

	// Create a new template using our main HTML string
	qrCodeTemplate, err := template.New("qr.code.generator").Parse(MAIN_HTML_TEMPLATE)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Execute the template / tpl passing in our HTML data elements and writing the results
	// to our response writer
	if err := qrCodeTemplate.Execute(w, htmlData); err != nil {
		fmt.Println(err)
	}

}

// Variables for handling our SVG drawing:

const (
	canvasWidth, canvasHeight = 800, 500
	numGridCells              = 100
	xyAxisRange               = 30.0                          // Axis ranges
	xyScale                   = canvasWidth / 2 / xyAxisRange // Pixels per x or y unit
	zScale                    = canvasHeight * 0.4            // Pixels per z unit
	angle                     = math.Pi / 6                   // Angle of x, y axes (=30°)
)

var sin30, cos30 = math.Sin(angle), math.Cos(angle) // sin(30°), cos(30°)

// This is our SVG drawing demo application. It computes an SVG rendering of a 3-D surface
// function. In our case below, we show an SVG rendering of sin(r)/r, where r is sqrt(x*x+y*y)
// The original example was taken from the book 'The Go Programming Langauge' and you can find it
// here: https://github.com/adonovan/gopl.io/blob/master/ch3/surface/main.go
func svgHandler(w http.ResponseWriter, r *http.Request) {

	// Since we don't want to pass in our HTML to our response writer quite yet, we store
	// the generated SVG results in memory via a bytes buffer
	var tpl bytes.Buffer

	// Below, we use our data / functions to construct the SVG drawing via standard XML notation
	fmt.Fprintf(&tpl, "<div class = \"main-content\">"+
		"<svg xmlns='http://www.w3.org/2000/svg' "+
		"style='stroke: grey; fill: white; stroke-width: 0.7' "+
		"width='%d' height='%d'>", canvasWidth, canvasHeight)

	for i := 0; i < numGridCells; i++ {
		for j := 0; j < numGridCells; j++ {
			ax, ay := corner(i+1, j)
			bx, by := corner(i, j)
			cx, cy := corner(i, j+1)
			dx, dy := corner(i+1, j+1)
			fmt.Fprintf(&tpl, "<polygon points='%g,%g %g,%g %g,%g %g,%g'/>\n",
				ax, ay, bx, by, cx, cy, dx, dy)
		}
	}

	fmt.Fprintln(&tpl, "</svg></div>")

	// Convert our encoded template data to a string
	bodyHTML := tpl.String()

	// Create the data elements we'll use to pass to our main HTML template
	htmlData := HtmlData{
		Title:       "Golang SVG Generation",
		Description: "Simple golang svg generation.",
		Keywords:    "golang web server svg generation",
		Author:      "",
		CssFiles: []string{
			"https://fonts.googleapis.com/css?family=Open+Sans",
		},
		CssScript:   template.HTML(MAIN_CSS_TEMPLATE),
		BodyContent: template.HTML(bodyHTML),
	}

	// Create a new template we'll use to display our SVG results using our main HTML string
	svgTemplate, err := template.New("svg").Parse(MAIN_HTML_TEMPLATE)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Execute the template / tpl passing in our HTML data elements and writing the results
	// to our response writer
	if err := svgTemplate.Execute(w, htmlData); err != nil {
		fmt.Println(err)
	}

}

// Methods used to construct our SVG surface drawing:

func corner(i, j int) (float64, float64) {

	// Find the point (x,y) at corner of cell (i, j)
	x := xyAxisRange * (float64(i)/numGridCells - 0.5)
	y := xyAxisRange * (float64(j)/numGridCells - 0.5)

	// Compute the surface height z
	z := surfaceHeight(x, y)

	// Project (x,y,z) isometrically onto a 2-D SVG canvas (sx,sy).
	sx := canvasWidth/2 + (x-y)*cos30*xyScale
	sy := canvasHeight/2 + (x+y)*sin30*xyScale - z*zScale

	return sx, sy

}

func surfaceHeight(x, y float64) float64 {
	// Get the total distance from (0,0)
	r := math.Hypot(x, y)
	// Return the z element / height
	return math.Sin(r) / r
}

// This is the raw Javascript we use to construct our rotating sphere in THREE.js. You can find
// the raw file in the js folder (titled sphere.js).
const THREE_JS_SPHERE_SCRIPT = `
<script>
	
	// Colour hex codes
	colors = { BLACK: 0x000000, WHITE: 0xffffff };

	// The main spherical properties we want to use
	var numberOfPoints = 250;
	var sphereRadius = 25;

	var pointCoordinates = generatePointCoordinates(numberOfPoints, sphereRadius);

	// The scene's local y rotation expressed in radians. This controls how quickly the
	// sphere rotates.
	var rotationSpeed = 0.008;

	// Generate and render the scene
	generateScene(pointCoordinates, rotationSpeed);

	// This function generates a list of world point coordinates evenly distributed on
	// the surface of our sphere and returns them.
	function generatePointCoordinates(numberOfPoints, sphereRadius) {
	var points = [];

	for (var i = 0; i < numberOfPoints; i++) {
		// Calculate the appropriate z increment / unit sphere z coordinate
		// so that we distribute our points evenly between the interval [-1, 1]
		var z_increment = 1 / numberOfPoints;
		var unit_sphere_z = 2 * i * z_increment - 1 + z_increment;

		// Calculate the unit sphere cross sectional radius cutting through the
		// x-y plane at point z
		var x_y_radius = Math.sqrt(1 - Math.pow(unit_sphere_z, 2));

		// Calculate the azimuthal angle (phi) so we can try to evenly distribute
		// our points on our spherical surface
		var phi_angle_increment = 2.4; // approximation of Math.PI * (3 - Math.sqrt(5));
		var phi = (i + 1) * phi_angle_increment;

		var unit_sphere_x = Math.cos(phi) * x_y_radius;
		var unit_sphere_y = Math.sin(phi) * x_y_radius;

		// Calculate the (x, y, z) world point coordinates
		x = unit_sphere_x * sphereRadius;
		y = unit_sphere_y * sphereRadius;
		z = unit_sphere_z * sphereRadius;

		var point = {
		x: x,
		y: y,
		z: z
		};

		points.push(point);
	}

	return points;
	}

	function generateScene(pointCoordinates, rotationSpeed) {
	var scene = new THREE.Scene();

	scene.background = new THREE.Color(colors.WHITE);

	// Frustum variables to use for the perspective camera
	var fieldOfView = 45;
	var aspect = window.innerWidth / window.innerHeight;
	var nearPlane = 1;
	var farPlane = 600;

	camera = new THREE.PerspectiveCamera(
		fieldOfView,
		aspect,
		nearPlane,
		farPlane
	);

	// Set the camera position to (x = 0, y = 0, z = 80) in world space.
	camera.position.x = 0;
	camera.position.y = 0;
	camera.position.z = 125;

	// Rotate the camera to face the point (x = 0, y = 0, z = 0) in world space.
	camera.lookAt(new THREE.Vector3(0, 0, 0));

	var renderer = new THREE.WebGLRenderer();
	renderer.setSize(window.innerWidth, window.innerHeight);

	// Add the renderer canvas (where the renderer draws its output) to the page.
	document.getElementById('sphere-container').appendChild(renderer.domElement);

	for (var i = 0; i < pointCoordinates.length; i++) {
		// Create the spherical point
		var pointRadius = 0.25;
		var geometry = new THREE.SphereGeometry(pointRadius);
		var material = new THREE.MeshBasicMaterial({ color: colors.BLACK });
		var point = new THREE.Mesh(geometry, material);

		// Set the point coordinates and add the point to our scene

		var pointCoordinate = pointCoordinates[i];

		point.position.x = pointCoordinate.x;
		point.position.y = pointCoordinate.y;
		point.position.z = pointCoordinate.z;

		scene.add(point);
		
	}

	function render() {
		// Set the scene y rotation to the appropriate speed and render the scene
		scene.rotation.y += rotationSpeed;
		requestAnimationFrame(render);
		renderer.render(scene, camera);
	}

	render();
	
	}

</script>
`

// This is a handler used to display a rotating sphere using THREE.js
func sphereHandler(w http.ResponseWriter, r *http.Request) {

	// Let's create the data elements we'll pass into our main template file
	htmlData := HtmlData{
		Title:       "Golang THREE.js Rotating Sphere",
		Description: "Simple golang THREE.js rotating sphere.",
		Keywords:    "golang web server THREE.js rotating sphere",
		Author:      "",
		CssFiles: []string{
			"https://fonts.googleapis.com/css?family=Open+Sans",
		},
		JsFiles: []string{
			"https://cdnjs.cloudflare.com/ajax/libs/three.js/103/three.min.js",
		},
		CssScript: template.HTML(MAIN_CSS_TEMPLATE),
		JsScript:  template.HTML(THREE_JS_SPHERE_SCRIPT),
		BodyContent: template.HTML(`
		<div id="table-container">
			<div id="container">
				<div id="main">
					<section id="sphere-container"></section>
				</div>
			</div>
		</div>
		`),
	}

	// Create a new template using our main HTML string and our raw THREE.js script
	sphereTemplate, err := template.New("sphere").Parse(MAIN_HTML_TEMPLATE)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Execute the template / tpl passing in our HTML data elements and writing the results
	// to our response writer
	if err := sphereTemplate.Execute(w, htmlData); err != nil {
		fmt.Println(err)
	}

}

// This is our log handler. It simply outputs our log file contents to the response writer
func logHandler(w http.ResponseWriter, r *http.Request) {

	// The below header settings prevent "mime" based attacks.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	// Read in our logging data file
	logData, err := ioutil.ReadFile(LOG_FILE_NAME)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write the log file data out to the response writer
	fmt.Fprintln(w, string(logData))

}

// Report server status
func healthHandler(w http.ResponseWriter, r *http.Request) {
	// Check our health state indicator, and if it's not OK, we return a status indicating that
	// our service is unavailable. Otherwise, we return a header with a 204 response code.
	if atomic.LoadInt32(&healthy) == 1 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}

// Returns a handler for our logging behavior
func loggingHandler(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Middleware layer we use to do our logging. In this instance, we defer
			// its execution to perform logging only after our main handler finishes
			// executing.
			defer func() {
				requestID, ok := r.Context().Value(REQUEST_ID_KEY).(string)
				// Check to see if we know which request we're handling
				if !ok {
					requestID = "UNKNOWN"
				}
				// Log the request info / details
				logger.Println(requestID, r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent())

			}()

			// Transfer control to the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// Returns a handler for our tracing
func tracingHandler(nextRequestID func() string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Let's try to get the header request ID
			requestID := r.Header.Get("X-Request-Id")
			// If one isn't assigned, we generate a new one
			if requestID == "" {
				requestID = nextRequestID()
			}
			// Create a new context with our request id value and key mapped to it
			ctx := context.WithValue(r.Context(), REQUEST_ID_KEY, requestID)
			// Add / set the header request id
			w.Header().Set("X-Request-Id", requestID)
			// Transfer control to the next handler with our newly created context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
