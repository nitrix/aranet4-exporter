package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
)

var addr string
var authUser string
var authPass string

func emitError(w http.ResponseWriter, name string, reason error) {
	fmt.Fprintf(w, "# HELP aranet4_error Aranet4 errors\n")
	fmt.Fprintf(w, "# TYPE aranet4_error counter\n")
	fmt.Fprintf(w, "aranet4_error{name=%q, reason=%q} 1\n", name, reason)
}

func emitMetricRow(w io.Writer, name string, value interface{}) {
	fmt.Fprintf(w, "%s %v\n", name, value)
}

func emitMetricSuccessfully(w io.Writer, cr CurrentReading) {
	// Success.
	fmt.Fprintf(w, "# HELP aranet4_success Aranet4 success\n")
	fmt.Fprintf(w, "# TYPE aranet4_success counter\n")
	fmt.Fprintf(w, "aranet4_success 1\n")

	// Temperature.
	fmt.Fprintf(w, "# HELP aranet4_temperature_c Temperature in Celsius\n")
	fmt.Fprintf(w, "# TYPE aranet4_temperature_c gauge\n")
	emitMetricRow(w, "aranet4_temperature_c", cr.Temperature)

	// CO2.
	fmt.Fprintf(w, "# HELP aranet4_co2_ppm CO2 in ppm\n")
	fmt.Fprintf(w, "# TYPE aranet4_co2_ppm gauge\n")
	emitMetricRow(w, "aranet4_co2_ppm", cr.CO2)

	// Battery.
	fmt.Fprintf(w, "# HELP aranet4_battery_percent Battery level in percent\n")
	fmt.Fprintf(w, "# TYPE aranet4_battery_percent gauge\n")
	emitMetricRow(w, "aranet4_battery_percent", cr.Battery)

	// Pressure.
	fmt.Fprintf(w, "# HELP aranet4_pressure_hpa Pressure in hPa\n")
	fmt.Fprintf(w, "# TYPE aranet4_pressure_hpa gauge\n")
	emitMetricRow(w, "aranet4_pressure_hpa", cr.Pressure)

	// Humidity.
	fmt.Fprintf(w, "# HELP aranet4_humidity_percent Humidity in percent\n")
	fmt.Fprintf(w, "# TYPE aranet4_humidity_percent gauge\n")
	emitMetricRow(w, "aranet4_humidity_percent", cr.Humidity)
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	a := NewAranet4()
	err := a.Connect(addr)
	if err != nil {
		emitError(w, "connect", err)
		return
	}

	currentReading, err := a.CurrentReading(true)
	if err != nil {
		emitError(w, "current_reading", err)
		return
	}

	fmt.Printf("Current reading: %+#v\n", currentReading)

	err = a.Disconnect()
	if err != nil {
		emitError(w, "disconnect", err)
		return
	}

	emitMetricSuccessfully(w, currentReading)
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

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatalf("could not start http server: %+v", err)
	}
}
