package config

import (
	"fmt"
	"log"
	"reflect"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/go-validator/validator"
	"github.com/zamN/zounce/user"
)

type Config struct {
	Title string
	Port  int
	Users map[string]user.User `validate:"nonzero,hasusers"`
}

type ConfigError struct {
	Field   string
	Message string
}

func (ce ConfigError) Error() string {
	return fmt.Sprintf("%s: %s", ce.Field, ce.Message)
}

type MultiError struct {
	Errors []error
}

func (me MultiError) Error() string {
	var errStr string
	for _, e := range me.Errors {
		errStr += ", " + e.Error()
	}
	errStr = strings.TrimLeft(errStr, ",")
	return fmt.Sprintf("%s", errStr)
}

type UserError struct {
	User    string
	Message string
}

func (ue UserError) Error() string {
	return fmt.Sprintf("[users.%s] -> %s", ue.User, ue.Message)
}

type NetworkError struct {
	Network string
	Message string
}

func (ne NetworkError) Error() string {
	return fmt.Sprintf("[networks.%s] -> %s", ne.Network, ne.Message)
}

var errorExpl = map[string]map[error]string{
	// TODO: Don't hardcode adapter 'valid options'
	"Logging.Adapter":  map[error]string{validator.ErrZeroValue: "An adapter is required. Valid Options: SQLite3, Flatfile"},
	"Logging.Database": map[error]string{validator.ErrZeroValue: "You must specify the name of the logging database."},
	"Nick":             map[error]string{validator.ErrZeroValue: "You must specify a nickname in order to connect to an IRC server.", validator.ErrMax: "Nickname can only be 9 characters long."},
	"AltNick":          map[error]string{validator.ErrZeroValue: "You must specify a alternate nickname in order to connect to an IRC server.", validator.ErrMax: "Altenate nickname can only be 9 characters long."},
	"Certs":            map[error]string{validator.ErrZeroValue: "You must specify at least one certificate in order to authenticate to zounce.", validator.ErrMin: "You must have at least 1 certificate on your user in order to authenticate."},
	"Users":            map[error]string{validator.ErrZeroValue: "You must specify at least one user in order to use to zounce."},
	"Servers":          map[error]string{validator.ErrMin: "You must specify at least one server in order to use this network with zounce."},
	"Name":             map[error]string{validator.ErrZeroValue: "You must specify a name for this network!"},
	"AuthInfo.CAPath":  map[error]string{validator.ErrZeroValue: "You must specify the CA for your certificate to verify."},
}

func validateNetworks(v interface{}, param string) error {
	st := reflect.ValueOf(v)

	var nError *NetworkError
	if st.Kind() == reflect.Map {
		keys := st.MapKeys()
		for _, server := range keys {
			isValid, errMap := validator.Validate(st.MapIndex(server).Interface())
			if !isValid {
				for k, v := range errMap {
					for _, err := range v {
						errorMsg := errorExpl[k][err]
						if len(errorMsg) > 0 {
							nError = &NetworkError{server.String(), errorMsg}
						} else {
							nError = &NetworkError{server.String(), fmt.Sprintf("Unknown error: %s", err)}
						}
					}
				}
			}
		}
	}

	// Not sure why I have to do this :p
	// returned 'nil' object still gets processed
	if nError == nil {
		return nil
	}
	return nError
}

func validateUsers(v interface{}, param string) error {
	st := reflect.ValueOf(v)

	var mError MultiError
	if st.Kind() == reflect.Map {
		keys := st.MapKeys()
		for _, user := range keys {
			isValid, errMap := validator.Validate(st.MapIndex(user).Interface())
			if !isValid {
				for k, v := range errMap {
					for _, err := range v {
						errorMsg := errorExpl[k][err]

						// If this is a top level error
						if len(errorMsg) > 0 {
							mError.Errors = append(mError.Errors, &UserError{user.String(), errorMsg})
						} else {
							switch reflect.TypeOf(err).String() {
							case "*config.NetworkError":
								ne := err.(*NetworkError)
								mError.Errors = append(mError.Errors, &UserError{user.String(), ne.Error()})
								break
							default:
								fmt.Println("TODO: Add new reflection type to switch statemtn lol")
								break
							}
						}
					}
				}
			}
		}
	}
	return mError
}

func LoadConfig(configFile string) (*Config, []error) {
	var c Config
	_, err := toml.DecodeFile(configFile, &c)
	if err != nil {
		log.Fatalf("Cannot load config file! Error: %s\n", err)
	}

	var errs []error

	validator.SetValidationFunc("hasusers", validateUsers)
	validator.SetValidationFunc("validnetworks", validateNetworks)
	isValid, errMap := validator.Validate(c)

	if !isValid {
		for k, v := range errMap {
			for _, err := range v {
				switch reflect.TypeOf(err).String() {
				// For dealing with sub-errors within config segments
				case "config.MultiError":
					errors := reflect.ValueOf(err).FieldByName("Errors")
					for i := 0; i < errors.Len(); i++ {
						ue := errors.Index(i).Interface().(*UserError)
						errs = append(errs, ue)
					}
					break
				case "*errors.errorString":
					kErr := errorExpl[k][err]
					if len(kErr) > 0 {
						errs = append(errs, &ConfigError{k, kErr})
					} else {
						errs = append(errs, &ConfigError{k, err.Error()})
					}
				default:
					fmt.Println("log this?")
					break
				}
			}
		}
	}

	// TODO: Config validation, default values, etc

	return &c, errs
}
