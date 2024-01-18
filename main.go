package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"runtime"
	"sync"
	"time"

	pb "co.com.devitech/pump/pumpstreaming/resources/output" // Import your generated gRPC code
	"google.golang.org/grpc"
)

type PacketData struct {
	Pump              int32   `json:"pump"`
	Face              int32   `json:"face"`
	Hose              int32   `json:"hose"`
	ProductFamily     string  `json:"productFamily"`
	Price             float64 `json:"price"`
	UnitSymbol        string  `json:"unitSymbol"`
	Amount            float64 `json:"amount"`
	Volume            float64 `json:"volume"`
	TransmissionType  string  `json:"transmissionType"`
	AuthorizationId   int64   `json:"authorizationId"`
	AuthorizationType string  `json:"authorizationType"`
	Authorized        bool    `json:"authorized"`
}

type server struct{}

var clients = make(map[pb.StreamingService_StreamDataServer]struct{})
var mu sync.Mutex

func (s *server) StreamData(req *pb.StreamRequest, stream pb.StreamingService_StreamDataServer) error {
	log.Println("CLIENTE SOLICITANTE: :::::> ", req.ClientID)
	mu.Lock()
	clients[stream] = struct{}{}
	mu.Unlock()

	for {
		select {
		// Simulate sending data to clients
		case <-stream.Context().Done():
			mu.Lock()
			delete(clients, stream)
			mu.Unlock()
			return nil
		}
	}
}

func broadcastData(data PacketData) {
	mu.Lock()
	defer mu.Unlock()
	var i int64 = 0
	for client := range clients {

		if err := client.Send(&pb.StreamResponse{
			Pump:              data.Pump,
			Face:              data.Face,
			Hose:              data.Hose,
			ProductFamily:     data.ProductFamily,
			Price:             data.Price,
			UnitSymbol:        data.UnitSymbol,
			Amount:            data.Amount,
			Volume:            data.Volume,
			TransmissionType:  data.TransmissionType,
			AuthorizationType: data.AuthorizationType,
			AuthorizationId:   data.AuthorizationId,
			Authorized:        data.Authorized,
		}); err != nil {
			fmt.Printf("Error sending data to client: %v\n", err)
		}
		log.Println("CLIENTE: ", i)
		i++
	}
}

var channelQueue = make(chan PacketData)

func main() {

	var GOMAXPROCS int = 10
	runtime.GOMAXPROCS(GOMAXPROCS)
	// ----------------------------------------UDP SERVER RECEIVE
	// UDP server setup
	udpAddr, err := net.ResolveUDPAddr("udp", "0.0.0.0:12345")
	if err != nil {
		fmt.Printf("Failed to resolve UDP address: %v\n", err)
		return
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		fmt.Printf("Failed to listen for UDP: %v\n", err)
		return
	}

	go func() {
		buf := make([]byte, 3072)
		for {
			n, _, err := udpConn.ReadFromUDP(buf)
			if err != nil {
				fmt.Printf("Error reading UDP packet: %v\n", err)
				continue
			}

			fmt.Printf("Received UDP data: %s\n", buf[:n])

			var parsedData PacketData
			if err := json.Unmarshal(buf[:n], &parsedData); err != nil {
				fmt.Println("Error al analizar los datos del paquete:", err)
				continue
			}

			// ADD PACKET TO QUEUE
			channelQueue <- parsedData
		}
	}()

	// ---------------------------- GRPC BROADCASTING
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		fmt.Printf("Failed to listen: %v\n", err)
		return
	}

	s := grpc.NewServer()
	pb.RegisterStreamingServiceServer(s, &server{})

	go func() {

		for {

			for data := range channelQueue {
				fmt.Printf("SENDING TO CLIENTS : %d\n", data)
				broadcastData(data)

			}

			time.Sleep(200 * time.Millisecond)
		}
	}()

	if err := s.Serve(lis); err != nil {
		fmt.Printf("Failed to serve: %v\n", err)
		return
	}
}
