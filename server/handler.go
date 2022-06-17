package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cmwaters/skychart/types"
	"github.com/gorilla/mux"
)

// Handler is the core object in the server package. It keeps an in-memory state
// of the chain-registry which can be updated using `Pull`. It handles requests
// for this data through the router.
type Handler struct {
	registryUrl  string
	lastUpdated  time.Time
	chains       []string
	assets       []string
	paths        []string
	chainByAsset map[string]string                  // asset name -> chain name
	chainById    map[string]string                  // chain id -> chain name
	pathsByTag   map[string]map[string][]types.Path // tag -> paths
	chainList    map[string]types.Chain
	assetList    map[string]types.AssetList
	pathList     map[string]types.Path
	log          *log.Logger
}

func NewHandler(registryUrl string, log *log.Logger) *Handler {
	pathsByTag := make(map[string]map[string][]types.Path)
	pathsByTag["dex"] = make(map[string][]types.Path)
	pathsByTag["preferred"] = make(map[string][]types.Path)
	pathsByTag["properties"] = make(map[string][]types.Path)
	pathsByTag["status"] = make(map[string][]types.Path)
	return &Handler{
		registryUrl:  registryUrl,
		lastUpdated:  time.Unix(0, 0),
		chains:       make([]string, 0),
		assets:       make([]string, 0),
		paths:        make([]string, 0),
		chainByAsset: make(map[string]string),
		chainById:    make(map[string]string),
		pathsByTag:   pathsByTag,
		chainList:    make(map[string]types.Chain),
		assetList:    make(map[string]types.AssetList),
		pathList:     make(map[string]types.Path),
		log:          log,
	}
}

func (h Handler) Chains(res http.ResponseWriter, req *http.Request) {
	respondWithJSON(res, h.chains)
}

// Chain searches for a chain by either name or ID and
// returns it if it exists
func (h Handler) Chain(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	chainName, ok := vars["chain"]
	if !ok {
		badRequest(res)
		return
	}

	exists, chain := h.findChain(chainName)
	if !exists {
		resourceNotFound(res)
		return
	}
	respondWithJSON(res, chain)
}

func (h Handler) Endpoints(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	chainName, ok := vars["chain"]
	if !ok {
		badRequest(res)
		return
	}
	endpointType, ok := vars["type"]
	if !ok {
		badRequest(res)
		return
	}
	exists, chain := h.findChain(chainName)
	if !exists {
		resourceNotFound(res)
		return
	}

	switch endpointType {
	case "rpc":
		respondWithJSON(res, chain.Apis.RPC)
	case "grpc":
		respondWithJSON(res, chain.Apis.Grpc)
	case "rest":
		respondWithJSON(res, chain.Apis.REST)
	case "peers":
		respondWithJSON(res, chain.Peers.PersistentPeers)
	case "seeds":
		respondWithJSON(res, chain.Peers.Seeds)
	default:
		badRequest(res)
	}
}

func (h Handler) ChainAsset(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	chainName, ok := vars["chain"]
	if !ok {
		badRequest(res)
		return
	}
	assets, ok := h.assetList[chainName]
	if !ok {
		chainName, ok = h.chainById[chainName]
		if !ok {
			badRequest(res)
		}
		assets = h.assetList[chainName]
	}
	respondWithJSON(res, assets)
}

func (h Handler) Assets(res http.ResponseWriter, req *http.Request) {
	respondWithJSON(res, h.assets)
}

func (h Handler) Asset(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	assetName, ok := vars["asset"]
	if !ok {
		badRequest(res)
		return
	}
	chainName, ok := h.chainByAsset[assetName]
	if !ok {
		resourceNotFound(res)
		return
	}

	assetList := h.assetList[chainName]
	for _, asset := range assetList.Assets {
		if asset.Display == assetName {
			respondWithJSON(res, asset)
			return
		}
	}

	resourceNotFound(res)
}

func (h Handler) PathNames(res http.ResponseWriter, req *http.Request) {
	respondWithJSON(res, h.paths)
}

func (h Handler) Paths(res http.ResponseWriter, req *http.Request) {
	paths := []types.Path{}

	for _, path := range h.pathList {
		paths = append(paths, path)
	}

	respondWithJSON(res, paths)
}

func (h Handler) PathsFiltered(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	dex, ok := vars["dex"]
	if ok {
		respondWithJSON(res, h.getPathsWithTag("dex", dex))
		return
	}
	preferred, ok := vars["preferred"]
	if ok {
		respondWithJSON(res, h.getPathsWithTag("preferred", preferred))
		return
	}
	properties, ok := vars["properties"]
	if ok {
		respondWithJSON(res, h.getPathsWithTag("properties", properties))
		return
	}
	status, ok := vars["status"]
	if ok {
		respondWithJSON(res, h.getPathsWithTag("status", status))
		return
	}

	// this should never be reached
	respondWithJSON(res, []types.Path{})
}

// Path searches for a path by chain name pair "{chain1Name}-{chain2Name}"
func (h Handler) Path(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	pathName, ok := vars["path"]
	if !ok {
		badRequest(res)
		return
	}

	// The router should reject input that doesn't match the "{chain1Name}-{chain2Name}" pattern
	chainNames := strings.Split(pathName, "-")
	pathName = h.getPathName(chainNames[0], chainNames[1])
	exists, path := h.findPath(pathName)
	if !exists {
		resourceNotFound(res)
		return
	}
	respondWithJSON(res, path)
}

func (h Handler) getPathsWithTag(tag string, value string) []types.Path {
	if len(value) == 0 {
		paths := []types.Path{}

		for _, path := range h.pathList {
			paths = append(paths, path)
		}

		return paths
	}
	byTag, ok := h.pathsByTag[tag]
	if !ok {
		panic(fmt.Sprintf("pathsByTag doesn't contain tag key \"%s\"", tag))
	}

	matches, ok := byTag[value]
	if !ok {
		return []types.Path{}
	}

	return matches
}

func (h Handler) getPathName(chain1Name string, chain2Name string) string {
	if strings.Compare(chain1Name, chain2Name) == 1 {
		return fmt.Sprintf("%s-%s", chain2Name, chain1Name)
	}

	return fmt.Sprintf("%s-%s", chain1Name, chain2Name)
}

func (h Handler) findChain(name string) (bool, types.Chain) {
	chain, ok := h.chainList[name]
	if ok {
		return true, chain
	}

	name, ok = h.chainById[name]
	if !ok {
		return false, types.Chain{}
	}

	return true, h.chainList[name]
}

func (h Handler) findPath(name string) (bool, types.Path) {
	path, ok := h.pathList[name]
	if !ok {
		return false, types.Path{}
	}

	return true, path
}

func respondWithJSON(w http.ResponseWriter, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, Accept, Content-Type, Access-Control-Allow-Headers, Authorization, X-Requested-With")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(response)
}

func resourceNotFound(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, Accept, Content-Type, Access-Control-Allow-Headers, Authorization, X-Requested-With")
	w.WriteHeader(http.StatusNotFound)
}

func badRequest(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, Accept, Content-Type, Access-Control-Allow-Headers, Authorization, X-Requested-With")
	w.WriteHeader(http.StatusBadRequest)
}
