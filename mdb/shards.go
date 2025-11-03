// Copyright 2020 Kuei-chun Chen. All rights reserved.

package mdb

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Shard store shard information
type Shard struct {
	ID      string         `bson:"_id"`
	Host    string         `bson:"host"`
	State   int            `bson:"state"`
	Servers []ClusterStats `bson:"servers"`
	Tags    []string       `bson:"tags"`
}

// GetShards return all shards from listShards command
func GetShards(client *mongo.Client) ([]Shard, error) {
	ctx := context.Background()
	var shardsInfo struct {
		Shards []Shard
	}
	if err := client.Database("admin").RunCommand(ctx, bson.D{{Key: "listShards", Value: 1}}).Decode(&shardsInfo); err != nil {
		return nil, err
	}
	sort.Slice(shardsInfo.Shards, func(i int, j int) bool {
		return shardsInfo.Shards[i].ID < shardsInfo.Shards[j].ID
	})
	return shardsInfo.Shards, nil
}

// GetAllShardURIs returns URIs of all replicas
func GetAllShardURIs(shards []Shard, connString connstring.ConnString) ([]string, error) {
	var list []string
	isSRV := false
	if strings.Index(connString.String(), "mongodb+srv") == 0 {
		isSRV = true
	}
	for _, shard := range shards {
		idx := strings.Index(shard.Host, "/")
		setName := shard.Host[:idx]
		hosts := shard.Host[idx+1:]
		ruri := "mongodb://"
		if connString.Username != "" {
			ruri += connString.Username + ":" + url.QueryEscape(connString.Password) + "@" + hosts
		} else {
			ruri += hosts
		}
		ruri += fmt.Sprintf(`/%v?replicaSet=%v`, connString.Database, setName)
		if !isSRV && connString.AuthSource != "" {
			ruri += "&authSource=" + connString.AuthSource
		} else if isSRV {
			ruri += "&authSource=admin&tls=true"
		}
		ruri += GetQueryParams(connString, false)
		list = append(list, ruri)
	}
	return list, nil
}

// GetAllServerURIs returns URIs of all mongo servers
func GetAllServerURIs(shards []Shard, connString connstring.ConnString) ([]string, error) {
	var list []string
	isSRV := false
	if strings.HasPrefix(connString.String(), "mongodb+srv") {
		isSRV = true
	}
	// Try to preserve Atlas hostname format by checking original connection string hosts
	originalHostsMap := make(map[string]string)
	originalHostsList := []string{}
	for _, origHost := range connString.Hosts {
		// Store original hostname without port for matching
		origHostname := origHost
		if idx := strings.Index(origHost, ":"); idx > 0 {
			origHostname = origHost[:idx]
		}
		originalHostsList = append(originalHostsList, origHostname)
		// Extract base hostname pattern for matching (e.g., "prod-jesse-shard-00-00" from "prod-jesse-shard-00-00-pri.wucdt.gcp.mongodb.net")
		if strings.Contains(origHostname, "-pri") {
			// Extract pattern before -pri
			if idx := strings.Index(origHostname, "-pri"); idx > 0 {
				baseHost := origHostname[:idx]
				// Also try matching without the domain prefix (e.g., just "prod-jesse-shard-00-00")
				domainIdx := strings.Index(baseHost, ".")
				if domainIdx > 0 {
					baseHostNoDomain := baseHost[:domainIdx]
					originalHostsMap[baseHostNoDomain] = origHostname
				}
				originalHostsMap[baseHost] = origHostname
			}
		}
	}

	for _, shard := range shards {
		idx := strings.Index(shard.Host, "/")
		setName := shard.Host[:idx]
		allHosts := shard.Host[idx+1:]
		hosts := strings.Split(allHosts, ",")
		for _, host := range hosts {
			// Extract hostname and port
			hostname := host
			port := ""
			if idx := strings.Index(host, ":"); idx > 0 {
				hostname = host[:idx]
				port = host[idx:]
			}
			
			// Try to match with original hosts and preserve -pri suffix for Atlas
			matched := false
			if len(originalHostsMap) > 0 || len(originalHostsList) > 0 {
				// Extract shard identifier pattern (e.g., "prod-jesse-shard-00-00" from various formats)
				// Handle cases like "prod-jesse-shard-00-00.wucdt.gcp.mongodb.net" or "prod-jesse-shard-00-00-wucdt.gcp.mongodb.net"
				baseParts := strings.Split(hostname, ".")
				var shardID string
				if len(baseParts) > 0 {
					firstPart := baseParts[0]
					// Extract shard identifier (e.g., "prod-jesse-shard-00-00" from "prod-jesse-shard-00-00-wucdt")
					if strings.Contains(firstPart, "-shard-") {
						// Remove domain suffixes like "-wucdt", "-pri", "-sec" for matching
						re := strings.NewReplacer("-wucdt", "", "-pri", "", "-sec", "")
						cleaned := re.Replace(firstPart)
						if idx := strings.Index(cleaned, "-shard-"); idx > 0 {
							afterShard := cleaned[idx+7:]
							// Extract shard ID (e.g., "prod-jesse-shard-00-00" from "prod-jesse-shard-00-00-wucdt")
							if len(afterShard) >= 5 && strings.Contains(afterShard[:5], "-") {
								shardID = cleaned[:idx+7+5]
							} else {
								shardID = cleaned
							}
						} else {
							shardID = cleaned
						}
					} else {
						shardID = firstPart
					}
					
					// Check if we have a matching original host with -pri
					if origHost, ok := originalHostsMap[shardID]; ok {
						// Use original hostname format
						hostname = origHost
						matched = true
					} else {
						// Try to find a similar hostname in original list by pattern matching
						for _, origHost := range originalHostsList {
							// Extract base from original (e.g., "prod-jesse-shard-00-00" from "prod-jesse-shard-00-00-pri.wucdt.gcp.mongodb.net")
							origBase := origHost
							if idx := strings.Index(origHost, "-pri"); idx > 0 {
								origBase = origHost[:idx]
							}
							if dotIdx := strings.Index(origBase, "."); dotIdx > 0 {
								origBase = origBase[:dotIdx]
							}
							// Extract shard ID from original too
							origShardID := origBase
							if strings.Contains(origBase, "-shard-") {
								re := strings.NewReplacer("-wucdt", "", "-pri", "", "-sec", "")
								cleaned := re.Replace(origBase)
								if idx := strings.Index(cleaned, "-shard-"); idx > 0 {
									afterShard := cleaned[idx+7:]
									if len(afterShard) >= 5 && strings.Contains(afterShard[:5], "-") {
										origShardID = cleaned[:idx+7+5]
									} else {
										origShardID = cleaned
									}
								}
							}
							// If shard IDs match, use the original format
							if origShardID == shardID {
								hostname = origHost
								matched = true
								break
							}
						}
					}
				}
			}
			
			// If no match found, try to add -pri suffix for Atlas hostnames
			if !matched && strings.Contains(hostname, "-shard-") && strings.Contains(hostname, ".gcp.mongodb.net") {
				// Check if -pri is missing and add it before the first dot
				if !strings.Contains(hostname, "-pri") {
					// Insert -pri before the domain segment (before first dot)
					// But first, remove any domain suffix from the first part (e.g., "-wucdt")
					parts := strings.Split(hostname, ".")
					if len(parts) > 0 {
						firstPart := parts[0]
						// Remove domain suffixes like "-wucdt" from the first part
						re := strings.NewReplacer("-wucdt", "")
						cleanedFirst := re.Replace(firstPart)
						// Reconstruct with -pri inserted before the first dot
						hostname = cleanedFirst + "-pri." + strings.Join(parts[1:], ".")
					} else if dotIdx := strings.Index(hostname, "."); dotIdx > 0 {
						hostname = hostname[:dotIdx] + "-pri" + hostname[dotIdx:]
					}
				}
			}
			
			// Reconstruct host with port if it was present
			finalHost := hostname
			if port != "" {
				finalHost = hostname + port
			}
			
			ruri := "mongodb://"
			if connString.Username != "" {
				ruri += fmt.Sprintf(`%v:%v@%v/?`, connString.Username, url.QueryEscape(connString.Password), finalHost)
			} else {
				ruri += fmt.Sprintf(`%v/?`, finalHost)
			}
			// Include replica set name for proper replica set discovery
			// The driver will discover other members from this single host
			ruri += fmt.Sprintf(`replicaSet=%v&`, setName)
			if isSRV {
				ruri += "authSource=admin&tls=true"
			} else {
				if connString.AuthSource != "" {
					ruri += "authSource=" + connString.AuthSource
				} else if connString.Username != "" {
					ruri += "authSource=admin"
				}
			}
			ruri += GetQueryParams(connString, true)
			list = append(list, ruri)
		}
	}
	return list, nil
}

// GetQueryParams returns partial connection string from ConnString
func GetQueryParams(connString connstring.ConnString, isConnectDirect bool) string {
	ruri := ""
	if connString.SSLSet {
		ruri += "&tls=true"
	}
	if connString.SSLCaFileSet {
		ruri += "&tlsCAFile=" + connString.SSLCaFile
	}
	if connString.SSLClientCertificateKeyFileSet {
		ruri += "&tlsCertificateKeyFile=" + connString.SSLClientCertificateKeyFile
	}
	if connString.SSLInsecureSet {
		ruri += "&tlsInsecure=true"
	}
	if connString.ReadPreference != "" && !isConnectDirect {
		ruri += "&readPreference=" + connString.ReadPreference
	}
	if len(connString.ReadPreferenceTagSets) > 0 && !isConnectDirect {
		ruri += "&readPreferenceTags="
		cnt := 0
		for _, amap := range connString.ReadPreferenceTagSets {
			for k, v := range amap {
				ruri += k + ":" + v
				if cnt > 0 {
					ruri += ","
				}
				cnt++
			}
		}
	}
	if connString.WNumberSet {
		ruri += fmt.Sprintf("&w=%v", connString.WNumber)
	} else if connString.WString != "" {
		ruri += "&w=" + connString.WString
	}
	if connString.RetryReadsSet {
		ruri += "&retryReads=true"
	}
	if connString.RetryWritesSet {
		ruri += "&retryWrites=true"
	}
	return ruri
}
