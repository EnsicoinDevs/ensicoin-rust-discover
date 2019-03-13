package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	discover "github.com/EnsicoinDevs/ensicoin-rust-discover/rpc"
	"github.com/bwmarrin/discordgo"
	"google.golang.org/grpc"
)

func main() {
	discoverService := NewDiscoverService()

	//grpc client initialisation
	go discoverService.launchGrpc()
	//discord bot initialisation
	data, err := ioutil.ReadFile("oath")
	if err != nil {
		log.Fatalf("Error reading token: %v", err)
	}
	token := string(data[:len(data)-1])
	discord, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalf("couldn't connect do Discord: %v", err)
	}

	discord.AddHandler(discoverService.discoverPeer)
	err = discord.Open()
	if err != nil {
		log.Fatalf("couldn't listen to Discord: %v", err)
	}

	fmt.Println("Bot is now running.  Press CTRL-C to exit.")

	//send message to ensicoin channel
	guilds, err := discord.UserGuilds(10, "", "")
	if err != nil {
		log.Fatalf("couldn't get guilds from bot: %v", err)
	}

	for i := 0; i < len(guilds); i++ {
		if guilds[i].Name == "Ensicoin" {
			guildID := guilds[i].ID
			guild, err := discord.Guild(guildID)
			if err != nil {
				log.Fatalf("Couldn't get Ensicoin Guild from bot: %v", err)
			}

			for j := 0; j < len(guild.Channels); j++ {
				if guild.Channels[j].Name == "ensicoin" {
					discord.ChannelMessageSend(guild.Channels[j].ID, "422021 hello_world_i_m_an_ensicoin_peer 127.0.0.1")
				}
			}
		}
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	discord.Close()
}

type DiscoverService struct {
	DiscoveredPeers chan discover.NewPeer
}

func NewDiscoverService() *DiscoverService {
	return &DiscoverService{
		DiscoveredPeers: make(chan discover.NewPeer),
	}
}

func (d *DiscoverService) discoverPeer(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	channel, err := s.Channel(m.ChannelID)
	if err != nil {
		log.Fatalf("error fetching the discord channel of a message: %v", err)
	}

	if channel.Name != "ensicoin" {
		return
	}

	content := m.Content
	str := strings.Split(content, "please_connect ")
	d.DiscoveredPeers <- discover.NewPeer{Address: str[1]}

}

func (d *DiscoverService) launchGrpc() {
	conn, err := grpc.Dial("localhost:2442", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Couldn't connect to Discovery server: %v", err)
	}
	defer conn.Close()

	var client = discover.NewDiscoverClient(conn)

	for peer := range d.DiscoveredPeers {
		p := peer

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		_, err = client.DiscoverPeer(ctx, &p)
		if err != nil {
			log.Fatalf("Problem sending peer: %v", err)
		}
	}
}
