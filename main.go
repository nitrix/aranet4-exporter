package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

var addr string
var authUser string
var authPass string

func emitError(w http.ResponseWriter, name string, reason error) {
	fmt.Fprintf(w, "# HELP aranet4_error Aranet4 errors\n")
	fmt.Fprintf(w, "# TYPE aranet4_error counter\n")
	fmt.Fprintf(w, "aranet4_error{name=%q, reason=%q} 1\n", name, reason)
}

func emitMetricRow(w io.Writer, name string, value interface{}, ts time.Time) {
	fmt.Fprintf(w, "%s %v %d\n", name, value, ts.UnixMilli())
}

func emitMetrics(w io.Writer, ds []Data) {
	// Success.
	fmt.Fprintf(w, "# HELP aranet4_success Aranet4 success\n")
	fmt.Fprintf(w, "# TYPE aranet4_success counter\n")
	fmt.Fprintf(w, "aranet4_success 1\n")

	// Temperature.
	fmt.Fprintf(w, "# HELP aranet4_temperature_c Temperature in Celsius\n")
	fmt.Fprintf(w, "# TYPE aranet4_temperature_c gauge\n")
	for _, d := range ds {
		emitMetricRow(w, "aranet4_temperature_c", d.T, d.Time)
	}

	// CO2.
	fmt.Fprintf(w, "# HELP aranet4_co2_ppm CO2 in ppm\n")
	fmt.Fprintf(w, "# TYPE aranet4_co2_ppm gauge\n")
	for _, d := range ds {
		emitMetricRow(w, "aranet4_co2_ppm", d.CO2, d.Time)
	}

	// Battery.
	fmt.Fprintf(w, "# HELP aranet4_battery_percent Battery level in percent\n")
	fmt.Fprintf(w, "# TYPE aranet4_battery_percent gauge\n")
	for _, d := range ds {
		emitMetricRow(w, "aranet4_battery_percent", d.Battery, d.Time)
	}

	// Pressure.
	fmt.Fprintf(w, "# HELP aranet4_pressure_hpa Pressure in hPa\n")
	fmt.Fprintf(w, "# TYPE aranet4_pressure_hpa gauge\n")
	for _, d := range ds {
		emitMetricRow(w, "aranet4_pressure_hpa", d.P, d.Time)
	}

	// Humidity.
	fmt.Fprintf(w, "# HELP aranet4_humidity_percent Humidity in percent\n")
	fmt.Fprintf(w, "# TYPE aranet4_humidity_percent gauge\n")
	for _, d := range ds {
		emitMetricRow(w, "aranet4_humidity_percent", d.H, d.Time)
	}
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	dev, err := NewDevice(r.Context(), addr)
	if err != nil {
		emitError(w, "new_device", err)
		log.Printf("could not create aranet4 client: %v", err)
		return
	}
	defer dev.Close()

	ds, err := dev.Read()
	if err != nil {
		emitError(w, "read", err)
		log.Printf("could not read device data: %v", err)
	}

	emitMetrics(w, []Data{ds})
}

func basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if ok {
			usernameHash := sha256.Sum256([]byte(username))
			passwordHash := sha256.Sum256([]byte(password))
			expectedUsernameHash := sha256.Sum256([]byte(authUser))
			expectedPasswordHash := sha256.Sum256([]byte(authPass))

			usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], expectedUsernameHash[:]) == 1)
			passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], expectedPasswordHash[:]) == 1)

			if usernameMatch && passwordMatch {
				next.ServeHTTP(w, r)
				return
			}
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

func main() {
	flag.StringVar(&addr, "addr", "", "aranet4 device address")
	flag.StringVar(&authUser, "authuser", "exporter", "username for basic auth")
	flag.StringVar(&authPass, "authpass", "changeme", "password for basic auth")
	flag.Parse()

	if addr == "" {
		flag.Usage()
		return
	}

	http.Handle("/metrics", basicAuth(handleMetrics))

	err := http.ListenAndServe(":9963", nil)
	if err != nil {
		log.Fatalf("could not start http server: %+v", err)
	}
}
