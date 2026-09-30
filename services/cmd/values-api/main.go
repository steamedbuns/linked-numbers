// Command values-api serves the REST API for values and reports. It is the
// only service that reads or writes the database.
package main

import "github.com/steamedbuns/linked-numbers/services/internal/service"

func main() {
	service.Service{Name: "values-api", DefaultAddr: ":8081"}.Main()
}
