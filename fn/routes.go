package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Sirupsen/logrus"
	fnclient "github.com/iron-io/functions_go/client"
	apiroutes "github.com/iron-io/functions_go/client/routes"
	"github.com/iron-io/functions_go/models"
	fnmodels "github.com/iron-io/functions_go/models"
	"github.com/jmoiron/jsonq"
	"github.com/urfave/cli"
)

type routesCmd struct {
	client *fnclient.Functions
}

func routes() cli.Command {

	r := routesCmd{client: apiClient()}

	return cli.Command{
		Name:      "routes",
		Usage:     "manage routes",
		ArgsUsage: "fn routes",
		Subcommands: []cli.Command{
			{
				Name:      "call",
				Usage:     "call a route",
				ArgsUsage: "`app` /path",
				Action:    r.call,
				Flags:     runflags(),
			},
			{
				Name:      "list",
				Aliases:   []string{"l"},
				Usage:     "list routes for `app`",
				ArgsUsage: "`app`",
				Action:    r.list,
			},
			{
				Name:      "create",
				Aliases:   []string{"c"},
				Usage:     "create a route in an `app`",
				ArgsUsage: "`app` /path [image]",
				Action:    r.create,
				Flags: []cli.Flag{
					cli.Int64Flag{
						Name:  "memory,m",
						Usage: "memory in MiB",
						Value: 128,
					},
					cli.StringFlag{
						Name:  "type,t",
						Usage: "route type - sync or async",
						Value: "sync",
					},
					cli.StringSliceFlag{
						Name:  "config,c",
						Usage: "route configuration",
					},
					cli.StringFlag{
						Name:  "format,f",
						Usage: "hot function IO format - json or http",
						Value: "",
					},
					cli.IntFlag{
						Name:  "max-concurrency",
						Usage: "maximum concurrency for hot function",
						Value: 1,
					},
					cli.DurationFlag{
						Name:  "timeout",
						Usage: "route timeout",
						Value: 30 * time.Second,
					},
				},
			},
			{
				Name:      "update",
				Aliases:   []string{"u"},
				Usage:     "update a route in an `app`",
				ArgsUsage: "`app` /path [image]",
				Action:    r.update,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "image,i",
						Usage: "image name",
					},
					cli.Int64Flag{
						Name:  "memory,m",
						Usage: "memory in MiB",
					},
					cli.StringFlag{
						Name:  "type,t",
						Usage: "route type - sync or async",
					},
					cli.StringSliceFlag{
						Name:  "config,c",
						Usage: "route configuration",
					},
					cli.StringSliceFlag{
						Name:  "headers",
						Usage: "route response headers",
					},
					cli.StringFlag{
						Name:  "format,f",
						Usage: "hot container IO format - json or http",
					},
					cli.IntFlag{
						Name:  "max-concurrency,mc",
						Usage: "maximum concurrency for hot container",
					},
					cli.DurationFlag{
						Name:  "timeout",
						Usage: "route timeout (eg. 30s)",
					},
				},
			},
			{
				Name:  "config",
				Usage: "operate a route configuration set",
				Subcommands: []cli.Command{
					{
						Name:      "set",
						Aliases:   []string{"s"},
						Usage:     "store a configuration key for this route",
						ArgsUsage: "`app` /path <key> <value>",
						Action:    r.configSet,
					},
					{
						Name:      "unset",
						Aliases:   []string{"u"},
						Usage:     "remove a configuration key for this route",
						ArgsUsage: "`app` /path <key>",
						Action:    r.configUnset,
					},
				},
			},
			{
				Name:      "delete",
				Aliases:   []string{"d"},
				Usage:     "delete a route from `app`",
				ArgsUsage: "`app` /path",
				Action:    r.delete,
			},
			{
				Name:      "inspect",
				Aliases:   []string{"i"},
				Usage:     "retrieve one or all routes properties",
				ArgsUsage: "`app` /path [property.[key]]",
				Action:    r.inspect,
			},
		},
	}
}

func call() cli.Command {
	r := routesCmd{client: apiClient()}

	return cli.Command{
		Name:      "call",
		Usage:     "call a remote function",
		ArgsUsage: "`app` /path",
		Flags:     runflags(),
		Action:    r.call,
	}
}

func (a *routesCmd) list(c *cli.Context) error {
	if len(c.Args()) < 1 {
		return errors.New("error: routes listing takes one argument: an app name")
	}

	appName := c.Args().Get(0)

	resp, err := a.client.Routes.GetAppsAppRoutes(&apiroutes.GetAppsAppRoutesParams{
		Context: context.Background(),
		App:     appName,
	})

	if err != nil {
		switch err.(type) {
		case *apiroutes.GetAppsAppRoutesNotFound:
			return fmt.Errorf("error: %v", err.(*apiroutes.GetAppsAppRoutesNotFound).Payload.Error.Message)
		case *apiroutes.GetAppsAppRoutesDefault:
			return fmt.Errorf("unexpected error: %v", err.(*apiroutes.GetAppsAppRoutesDefault).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %v", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0)
	fmt.Fprint(w, "path", "\t", "image", "\t", "endpoint", "\n")
	for _, route := range resp.Payload.Routes {
		u, err := url.Parse("../")
		u.Path = path.Join(u.Path, "r", appName, route.Path)
		if err != nil {
			return fmt.Errorf("error parsing functions route path: %v", err)
		}

		fmt.Fprint(w, route.Path, "\t", route.Image, "\n")
	}
	w.Flush()

	return nil
}

func (a *routesCmd) call(c *cli.Context) error {
	if len(c.Args()) < 2 {
		return errors.New("error: routes listing takes three arguments: an app name and a path")
	}

	appName := c.Args().Get(0)
	route := c.Args().Get(1)

	u := url.URL{
		Scheme: "http",
		Host:   host(),
	}
	u.Path = path.Join(u.Path, "r", appName, route)
	content := stdin()

	return callfn(u.String(), content, os.Stdout, c.String("method"), c.StringSlice("e"))
}

func callfn(u string, content io.Reader, output io.Writer, method string, env []string) error {
	if method == "" {
		if content == nil {
			method = "GET"
		} else {
			method = "POST"
		}
	}

	req, err := http.NewRequest(method, u, content)
	if err != nil {
		return fmt.Errorf("error running route: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if len(env) > 0 {
		envAsHeader(req, env)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error running route: %v", err)
	}

	io.Copy(output, resp.Body)

	return nil
}

func envAsHeader(req *http.Request, selectedEnv []string) {
	detectedEnv := os.Environ()
	if len(selectedEnv) > 0 {
		detectedEnv = selectedEnv
	}

	for _, e := range detectedEnv {
		kv := strings.Split(e, "=")
		name := kv[0]
		req.Header.Set(name, os.Getenv(name))
	}
}

func (a *routesCmd) create(c *cli.Context) error {
	// todo: @pedro , why aren't you just checking the length here?
	if len(c.Args()) < 2 {
		return errors.New("error: routes listing takes at least two arguments: an app name and a path")
	}

	appName := c.Args().Get(0)
	route := c.Args().Get(1)
	image := c.Args().Get(2)
	var (
		format  string
		maxC    int
		timeout time.Duration
	)
	if image == "" {
		// todo: why do we only load the func file if image isn't set?  Don't we need to read the rest of these things regardless?
		ff, err := loadFuncfile()
		if err != nil {
			if _, ok := err.(*notFoundError); ok {
				return errors.New("error: image name is missing or no function file found")
			}
			return err
		}
		image = ff.FullName()
		if ff.Format != nil {
			format = *ff.Format
		}
		if ff.maxConcurrency != nil {
			maxC = *ff.maxConcurrency
		}
		if ff.Timeout != nil {
			timeout = *ff.Timeout
		}
		if route == "" && ff.path != nil {
			route = *ff.path
		}
	}

	if route == "" {
		return errors.New("error: route path is missing")
	}
	if image == "" {
		return errors.New("error: function image name is missing")
	}

	if f := c.String("format"); f != "" {
		format = f
	}
	if m := c.Int("max-concurrency"); m > 0 {
		maxC = m
	}
	if t := c.Duration("timeout"); t > 0 {
		timeout = t
	}

	to := int64(timeout.Seconds())
	body := &models.RouteWrapper{
		Route: &models.Route{
			Path:           route,
			Image:          image,
			Memory:         c.Int64("memory"),
			Type:           c.String("type"),
			Config:         extractEnvConfig(c.StringSlice("config")),
			Format:         format,
			MaxConcurrency: int32(maxC),
			Timeout:        &to,
		},
	}

	resp, err := a.client.Routes.PostAppsAppRoutes(&apiroutes.PostAppsAppRoutesParams{
		Context: context.Background(),
		App:     appName,
		Body:    body,
	})

	if err != nil {
		switch err.(type) {
		case *apiroutes.PostAppsAppRoutesBadRequest:
			return fmt.Errorf("error: %v", err.(*apiroutes.PostAppsAppRoutesBadRequest).Payload.Error.Message)
		case *apiroutes.PostAppsAppRoutesConflict:
			return fmt.Errorf("error: %v", err.(*apiroutes.PostAppsAppRoutesConflict).Payload.Error.Message)
		case *apiroutes.PostAppsAppRoutesDefault:
			return fmt.Errorf("unexpected error: %v", err.(*apiroutes.PostAppsAppRoutesDefault).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %v", err)
	}

	fmt.Println(resp.Payload.Route.Path, "created with", resp.Payload.Route.Image)
	return nil
}

func (a *routesCmd) patchRoute(appName, routePath string, r *fnmodels.Route) error {
	resp, err := a.client.Routes.GetAppsAppRoutesRoute(&apiroutes.GetAppsAppRoutesRouteParams{
		Context: context.Background(),
		App:     appName,
		Route:   routePath,
	})

	if err != nil {
		switch err.(type) {
		case *apiroutes.GetAppsAppRoutesRouteNotFound:
			return fmt.Errorf("error: %v", err.(*apiroutes.GetAppsAppRoutesRouteNotFound).Payload.Error.Message)
		case *apiroutes.GetAppsAppRoutesDefault:
			return fmt.Errorf("unexpected error: %v", err.(*apiroutes.GetAppsAppRoutesDefault).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %v", err)
	}

	if resp.Payload.Route.Config == nil {
		resp.Payload.Route.Config = map[string]string{}
	}

	if resp.Payload.Route.Headers == nil {
		resp.Payload.Route.Headers = map[string][]string{}
	}

	resp.Payload.Route.Path = ""
	if r != nil {
		if r.Config != nil {
			for k, v := range r.Config {
				if string(k[0]) == "-" {
					delete(resp.Payload.Route.Config, string(k[1:]))
					continue
				}
				resp.Payload.Route.Config[k] = v
			}
		}
		if r.Headers != nil {
			for k, v := range r.Headers {
				if string(k[0]) == "-" {
					delete(resp.Payload.Route.Headers, k)
					continue
				}
				resp.Payload.Route.Headers[k] = v
			}
		}
		if r.Image != "" {
			resp.Payload.Route.Image = r.Image
		}
		if r.Format != "" {
			resp.Payload.Route.Format = r.Format
		}
		if r.Type != "" {
			resp.Payload.Route.Type = r.Type
		}
		if r.MaxConcurrency > 0 {
			resp.Payload.Route.MaxConcurrency = r.MaxConcurrency
		}
		if r.Memory > 0 {
			resp.Payload.Route.Memory = r.Memory
		}
		if r.Timeout != nil {
			resp.Payload.Route.Timeout = r.Timeout
		}
	}

	_, err = a.client.Routes.PatchAppsAppRoutesRoute(&apiroutes.PatchAppsAppRoutesRouteParams{
		Context: context.Background(),
		App:     appName,
		Route:   routePath,
		Body:    resp.Payload,
	})

	if err != nil {
		switch err.(type) {
		case *apiroutes.PatchAppsAppRoutesRouteBadRequest:
			return fmt.Errorf("error: %v", err.(*apiroutes.PatchAppsAppRoutesRouteBadRequest).Payload.Error.Message)
		case *apiroutes.PatchAppsAppRoutesRouteNotFound:
			return fmt.Errorf("error: %v", err.(*apiroutes.PatchAppsAppRoutesRouteNotFound).Payload.Error.Message)
		case *apiroutes.PatchAppsAppRoutesRouteDefault:
			return fmt.Errorf("unexpected error: %v", err.(*apiroutes.PatchAppsAppRoutesRouteDefault).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %v", err)
	}

	return nil
}

func (a *routesCmd) update(c *cli.Context) error {
	if len(c.Args()) < 2 {
		return errors.New("error: route update takes at least two arguments: an app name and a path")
	}

	appName := c.Args().Get(0)
	route := c.Args().Get(1)
	image := c.Args().Get(2)
	var (
		format  string
		maxC    int
		timeout time.Duration
	)
	ff, err := loadFuncfile()
	if err != nil {
		if _, ok := err.(*notFoundError); ok {
			if image == "" {
				// the no image flag or func file
				return errors.New("error: image name is missing or no function file found")
			}
			logrus.Warnln("func file not found, continuing...")
		} else {
			return err
		}
	}
	if image != "" { // flags take precedence
		image = ff.FullName()
	}
	if ff.Format != nil {
		format = *ff.Format
	}
	if ff.maxConcurrency != nil {
		maxC = *ff.maxConcurrency
	}
	if ff.Timeout != nil {
		timeout = *ff.Timeout
	}
	if route == "" && ff.path != nil {
		route = *ff.path
	}

	if route == "" {
		return errors.New("error: route path is missing")
	}
	// if image == "" {
	// return errors.New("error: function image name is missing")
	// }

	if f := c.String("format"); f != "" {
		format = f
	}
	if m := c.Int("max-concurrency"); m > 0 {
		maxC = m
	}
	if t := c.Duration("timeout"); t > 0 {
		timeout = t
	}

	headers := map[string][]string{}
	for _, header := range c.StringSlice("headers") {
		parts := strings.Split(header, "=")
		headers[parts[0]] = strings.Split(parts[1], ";")
	}

	to := int64(timeout.Seconds())
	patchRoute := &fnmodels.Route{
		Image:          image,
		Memory:         c.Int64("memory"),
		Type:           c.String("type"),
		Config:         extractEnvConfig(c.StringSlice("config")),
		Headers:        headers,
		Format:         format,
		MaxConcurrency: int32(maxC),
		Timeout:        &to,
	}

	err = a.patchRoute(appName, route, patchRoute)
	if err != nil {
		return err
	}

	fmt.Println(appName, route, "updated")
	return nil
}

func (a *routesCmd) configSet(c *cli.Context) error {
	if len(c.Args()) < 4 {
		return errors.New("error: route configuration updates tak four arguments: an app name, a path, a key and a value")
	}

	appName := c.Args().Get(0)
	route := c.Args().Get(1)
	key := c.Args().Get(2)
	value := c.Args().Get(3)

	patchRoute := fnmodels.Route{
		Config: make(map[string]string),
	}

	patchRoute.Config[key] = value

	err := a.patchRoute(appName, route, &patchRoute)
	if err != nil {
		return err
	}

	fmt.Println(appName, route, "updated", key, "with", value)
	return nil
}

func (a *routesCmd) configUnset(c *cli.Context) error {
	if len(c.Args()) < 3 {
		return errors.New("error: route configuration updates take three arguments: an app name, a path and a key")
	}

	appName := c.Args().Get(0)
	route := c.Args().Get(1)
	key := c.Args().Get(2)

	patchRoute := fnmodels.Route{
		Config: make(map[string]string),
	}

	patchRoute.Config["-"+key] = ""

	err := a.patchRoute(appName, route, &patchRoute)
	if err != nil {
		return err
	}

	fmt.Printf("removed key '%s' from the route '%s%s'", key, appName, key)
	return nil
}

func (a *routesCmd) inspect(c *cli.Context) error {
	if len(c.Args()) < 2 {
		return errors.New("error: routes listing takes three arguments: an app name and a path")
	}

	appName := c.Args().Get(0)
	route := c.Args().Get(1)
	prop := c.Args().Get(2)

	resp, err := a.client.Routes.GetAppsAppRoutesRoute(&apiroutes.GetAppsAppRoutesRouteParams{
		Context: context.Background(),
		App:     appName,
		Route:   route,
	})

	if err != nil {
		switch err.(type) {
		case *apiroutes.GetAppsAppRoutesRouteNotFound:
			return fmt.Errorf("error: %v", err.(*apiroutes.GetAppsAppRoutesRouteNotFound).Payload.Error.Message)
		case *apiroutes.GetAppsAppRoutesRouteDefault:
			return fmt.Errorf("unexpected error: %v", err.(*apiroutes.GetAppsAppRoutesRouteDefault).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %v", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "\t")

	if prop == "" {
		enc.Encode(resp.Payload.Route)
		return nil
	}

	data, err := json.Marshal(resp.Payload.Route)
	if err != nil {
		return fmt.Errorf("failed to inspect route: %v", err)
	}
	var inspect map[string]interface{}
	err = json.Unmarshal(data, &inspect)
	if err != nil {
		return fmt.Errorf("failed to inspect route: %v", err)
	}

	jq := jsonq.NewQuery(inspect)
	field, err := jq.Interface(strings.Split(prop, ".")...)
	if err != nil {
		return errors.New("failed to inspect that route's field")
	}
	enc.Encode(field)

	return nil
}

func (a *routesCmd) delete(c *cli.Context) error {
	if len(c.Args()) < 2 {
		return errors.New("error: routes delete takes two arguments: an app name and a path")
	}

	appName := c.Args().Get(0)
	route := c.Args().Get(1)

	_, err := a.client.Routes.DeleteAppsAppRoutesRoute(&apiroutes.DeleteAppsAppRoutesRouteParams{
		Context: context.Background(),
		App:     appName,
		Route:   route,
	})
	if err != nil {
		switch err.(type) {
		case *apiroutes.DeleteAppsAppRoutesRouteNotFound:
			return fmt.Errorf("error: %v", err.(*apiroutes.DeleteAppsAppRoutesRouteNotFound).Payload.Error.Message)
		case *apiroutes.DeleteAppsAppRoutesRouteDefault:
			return fmt.Errorf("unexpected error: %v", err.(*apiroutes.DeleteAppsAppRoutesRouteDefault).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %v", err)
	}

	fmt.Println(appName, route, "deleted")
	return nil
}
