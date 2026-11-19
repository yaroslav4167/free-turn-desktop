package serversetup

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

type dockerPeer struct {
	Name      string `json:"name"`
	PublicKey string `json:"pub"`
	ClientID  string `json:"client_id"`
	IP        string `json:"ip"`
	Self      bool   `json:"self"`
	Handshake int64  `json:"hs"`
}

func dockerPeers(cfg SSHConfig) ([]dockerPeer, *ShareInfo, error) {
	r, err := dockerRPC(cfg, "client-list")
	if err != nil {
		return nil, nil, err
	}
	var d struct {
		Clients []dockerPeer `json:"clients"`
		Share   struct {
			Mode       string `json:"mode"`
			ObfProfile string `json:"obf_profile"`
			ObfKey     string `json:"obf_key"`
		} `json:"share"`
	}
	if err := json.Unmarshal(r.Data, &d); err != nil {
		return nil, nil, err
	}
	return d.Clients, &ShareInfo{Known: true, Mode: d.Share.Mode, ObfProfile: d.Share.ObfProfile, ObfKey: d.Share.ObfKey}, nil
}

func dockerShareList(cfg SSHConfig) (*PeerList, error) {
	peers, _, err := dockerPeers(cfg)
	if err != nil {
		return nil, err
	}
	clock, err := exec(cfg, "date +%s", "")
	if err != nil {
		return nil, err
	}
	now, err := strconv.ParseInt(strings.TrimSpace(clock.output), 10, 64)
	if err != nil || now <= 0 {
		return nil, errors.New("сервер не вернул текущее время")
	}
	list := &PeerList{Peers: []Peer{}, ServerNow: now}
	for _, p := range peers {
		if p.PublicKey == "" {
			continue
		}
		list.Peers = append(list.Peers, Peer{PublicKey: p.PublicKey, Name: p.Name, IP: p.IP, LastHandshake: p.Handshake, HasConf: true, IsOwner: p.Self})
	}
	return list, nil
}

func dockerIssue(cfg SSHConfig, command, name string) (*PeerIssued, error) {
	r, err := dockerRPC(cfg, command, "--name="+name)
	if err != nil {
		return nil, err
	}
	var d struct {
		Client dockerPeer `json:"client"`
		Conf   string     `json:"conf_b64"`
	}
	if err := json.Unmarshal(r.Data, &d); err != nil {
		return nil, err
	}
	conf, err := decodeConf(d.Conf)
	if err != nil {
		return nil, err
	}
	return &PeerIssued{PublicKey: d.Client.PublicKey, ClientID: d.Client.ClientID, IP: d.Client.IP, Conf: conf}, nil
}

func dockerPeerByKey(cfg SSHConfig, pub string) (*dockerPeer, error) {
	peers, _, err := dockerPeers(cfg)
	if err != nil {
		return nil, err
	}
	for _, p := range peers {
		if p.PublicKey == pub && pub != "" {
			return &p, nil
		}
	}
	return nil, errors.New("пользователь с этим публичным ключом не найден")
}
