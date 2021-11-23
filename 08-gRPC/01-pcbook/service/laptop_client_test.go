package service_test

import (
	"context"
	"io"
	"net"
	"pcbook/pb"
	"pcbook/sample"
	"pcbook/serializer"
	"pcbook/service"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestClientCreateLaptop(t *testing.T) {
	t.Parallel()

	laptopServer, serverAddress := startTestLaptopServer(t, service.NewInMemoryLaptopStore())
	laptopClient := newTestLaptopClient(t, serverAddress)

	laptop := sample.NewLaptop()
	expectedID := laptop.Id
	req := &pb.CreateLaptopRequest{
		Laptop: laptop,
	}

	res, err := laptopClient.CreateLaptop(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, expectedID, res.Id)

	// check that the laptop is saved to the store
	other, err := laptopServer.Store.Find(res.Id)
	require.NoError(t, err)
	require.NotNil(t, other)

	// check that the saved laptop is the same as the one we send
	requireSameLaptop(t, laptop, other)
}

//create a new laptop server with an in-memory laptop store
func startTestLaptopServer(t *testing.T, store service.LaptopStore) (*service.LaptopServer, string) {
	laptopServer := service.NewLaptopServer(store)

	//We create the gRPC server by calling grpc.NewServer() function, then register the laptop service server on that gRPC server.
	grpcServer := grpc.NewServer()
	pb.RegisterLaptopServiceServer(grpcServer, laptopServer)

	//We create a new listener that will listen to tcp connection.
	//The number 0 here means that we want it to be assigned any random available port.
	listener, err := net.Listen("tcp", ":0") // random available port
	require.NoError(t, err)

	go grpcServer.Serve(listener)

	return laptopServer, listener.Addr().String()
}

//return a new laptop-client
func newTestLaptopClient(t *testing.T, serverAddress string) pb.LaptopServiceClient {

	//First we dial the server address with grpc.Dial(). Since this is just for testing, we use an insecure connection.
	conn, err := grpc.Dial(serverAddress, grpc.WithInsecure())
	require.NoError(t, err)
	return pb.NewLaptopServiceClient(conn)
}

func TestClientSearchLaptop(t *testing.T) {
	t.Parallel()

	//First I will create a search filter and an in-memory laptop store to insert some laptops for searching
	filter := &pb.Filter{
		MaxPriceUsd: 2000,
		MinCpuCores: 4,
		MinCpuGhz:   2.2,
		MinRam:      &pb.Memory{Value: 8, Unit: pb.Memory_GIGABYTE},
	}

	store := service.NewInMemoryLaptopStore()

	//Then I make an expectedIDs map that will contain all laptop IDs that we expect to be found by the server, Case 4 + 5: matched.
	expectedIDs := make(map[string]bool)

	for i := 0; i < 6; i++ {
		laptop := sample.NewLaptop()

		switch i {
		case 0:
			laptop.PriceUsd = 2500
		case 1:
			laptop.Cpu.NumberCores = 2
		case 2:
			laptop.Cpu.MinGhz = 2.0
		case 3:
			laptop.Ram = &pb.Memory{Value: 4096, Unit: pb.Memory_MEGABYTE}
		case 4:
			laptop.PriceUsd = 1999
			laptop.Cpu.NumberCores = 4
			laptop.Cpu.MinGhz = 2.5
			laptop.Cpu.MaxGhz = laptop.Cpu.MinGhz + 2.0
			laptop.Ram = &pb.Memory{Value: 16, Unit: pb.Memory_GIGABYTE}
			expectedIDs[laptop.Id] = true
		case 5:
			laptop.PriceUsd = 2000
			laptop.Cpu.NumberCores = 6
			laptop.Cpu.MinGhz = 2.8
			laptop.Cpu.MaxGhz = laptop.Cpu.MinGhz + 2.0
			laptop.Ram = &pb.Memory{Value: 64, Unit: pb.Memory_GIGABYTE}
			expectedIDs[laptop.Id] = true
		}

		err := store.Save(laptop)
		require.NoError(t, err)
	}

	//Then call this function to start the test server, and create a laptop client object with that server address
	_, serverAddress := startTestLaptopServer(t, store)
	laptopClient := newTestLaptopClient(t, serverAddress)

	//After that, we create a new SearchLaptopRequest with the filter
	req := &pb.SearchLaptopRequest{Filter: filter}
	//Then we call laptopCient.SearchLaptop() with the created request to get back the stream. There should be no errors returned
	stream, err := laptopClient.SearchLaptop(context.Background(), req)
	require.NoError(t, err)

	//Next, I will use the found variable to keep track of the number of laptops found
	found := 0
	//Then use a for loop to receive multiple responses from the stream.
	for {
		res, err := stream.Recv()
		//If we got an end-of-file error, then break.
		if err == io.EOF {
			break
		}

		//Else we check that there’s no error, and the laptop ID should be in the expectedIDs map.
		require.NoError(t, err)
		require.Contains(t, expectedIDs, res.GetLaptop().GetId())

		//Then we increase the number of laptops found
		found += 1
	}

	//Finally we require that number to equal to the size of the expectedIDs.
	require.Equal(t, len(expectedIDs), found)
}

func requireSameLaptop(t *testing.T, laptop1 *pb.Laptop, laptop2 *pb.Laptop) {
	json1, err := serializer.ProtobufToJSON(laptop1)
	require.NoError(t, err)

	json2, err := serializer.ProtobufToJSON(laptop2)
	require.NoError(t, err)

	require.Equal(t, json1, json2)
}