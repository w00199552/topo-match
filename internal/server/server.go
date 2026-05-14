package server

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/w00199552/topo-match/internal/allocator"
	"github.com/w00199552/topo-match/internal/model"
)

// Server provides REST API for topo-match
type Server struct {
	app       *fiber.App
	allocator *allocator.Allocator
}

// NewServer creates a new Server
func NewServer() *Server {
	s := &Server{
		app: fiber.New(fiber.Config{
			AppName: "topo-match",
		}),
		allocator: allocator.NewAllocator(),
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	api := s.app.Group("/api/v1")

	// Testbed management
	api.Post("/testbeds", s.addTestbed)
	api.Delete("/testbeds/:name", s.removeTestbed)
	api.Get("/testbeds", s.listTestbeds)
	api.Get("/testbeds/:name/status", s.getTestbedStatus)

	// Allocation
	api.Post("/alloc", s.alloc)
	api.Post("/free", s.free)
}

// --- Request/Response types ---

type addTestbedRequest struct {
	Name     string `json:"name"`
	FilePath string `json:"file_path"` // local XML file path
}

type allocRequest struct {
	LogicFilePath string   `json:"logic_file_path"` // local XML file path
	TestbedNames  []string `json:"testbed_names"`
}

type freeRequest struct {
	AllocResult *allocator.AllocResult `json:"alloc_result"`
}

type response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// --- Handlers ---

func (s *Server) addTestbed(c *fiber.Ctx) error {
	req := new(addTestbedRequest)
	if err := c.BodyParser(req); err != nil {
		return c.JSON(response{Success: false, Error: err.Error()})
	}

	topo, err := model.ParseXMLFile(req.FilePath)
	if err != nil {
		return c.JSON(response{Success: false, Error: fmt.Sprintf("parse xml: %v", err)})
	}

	s.allocator.AddTestbed(req.Name, topo)
	return c.JSON(response{Success: true, Data: req.Name})
}

func (s *Server) removeTestbed(c *fiber.Ctx) error {
	name := c.Params("name")
	s.allocator.RemoveTestbed(name)
	return c.JSON(response{Success: true})
}

func (s *Server) listTestbeds(c *fiber.Ctx) error {
	names := s.allocator.ListTestbeds()
	return c.JSON(response{Success: true, Data: names})
}

func (s *Server) getTestbedStatus(c *fiber.Ctx) error {
	name := c.Params("name")
	status, err := s.allocator.GetTestbedStatus(name)
	if err != nil {
		return c.JSON(response{Success: false, Error: err.Error()})
	}
	return c.JSON(response{Success: true, Data: status})
}

func (s *Server) alloc(c *fiber.Ctx) error {
	req := new(allocRequest)
	if err := c.BodyParser(req); err != nil {
		return c.JSON(response{Success: false, Error: err.Error()})
	}

	logic, err := model.ParseXMLFile(req.LogicFilePath)
	if err != nil {
		return c.JSON(response{Success: false, Error: fmt.Sprintf("parse logic xml: %v", err)})
	}

	result, err := s.allocator.Alloc(logic, req.TestbedNames)
	if err != nil {
		return c.JSON(response{Success: false, Error: err.Error()})
	}

	return c.JSON(response{Success: true, Data: result})
}

func (s *Server) free(c *fiber.Ctx) error {
	req := new(freeRequest)
	if err := c.BodyParser(req); err != nil {
		return c.JSON(response{Success: false, Error: err.Error()})
	}

	if err := s.allocator.Free(req.AllocResult); err != nil {
		return c.JSON(response{Success: false, Error: err.Error()})
	}

	return c.JSON(response{Success: true})
}

// Run starts the HTTP server
func (s *Server) Run(addr string) error {
	return s.app.Listen(addr)
}
