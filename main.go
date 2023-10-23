package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"errors"
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"math/rand"
	"time"
)

type iTrie interface {
	Get(key []byte) ([]byte, error)
	Put(key []byte, value []byte)
	Del(key []byte) error
	Commit() []byte
	Proof(key []byte) ([][]byte, error)
}

type RadixTrie struct {
	root       *Node
	db         *gorm.DB
	dirtyNodes map[string]*Node
}

type Node struct {
	key      []byte
	value    []byte
	children map[byte]*Node
}

type Entry struct {
	gorm.Model
	Key   string `gorm:"unique"`
	Value []byte `gorm:"type:blob"`
}

func NewDatabase() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	db.AutoMigrate(&Entry{})

	return db
}

func (t *RadixTrie) Commit() []byte {
	for key, node := range t.dirtyNodes {
		encodedNode, _ := encodeNode(node)
		entry := Entry{Key: key, Value: encodedNode}
		t.db.Create(&entry)
		delete(t.dirtyNodes, key)
	}
	rootKey, _ := encodeNode(t.root)
	return rootKey
}

// Implement the iTrie interface for RadixTrie

func (t *RadixTrie) Get(key []byte) ([]byte, error) {
	node, err := t.findNode(key)
	if err != nil {
		return nil, err
	}
	return node.value, nil
}

func (t *RadixTrie) Put(key []byte, value []byte) {
	node, _ := t.findNode(key)
	if node == nil {
		t.root = t.insertNode(t.root, key, value)
		t.dirtyNodes[string(key)] = t.root
	} else {
		node.value = value
		t.dirtyNodes[string(key)] = node
	}
}

func (t *RadixTrie) Del(key []byte) error {
	_, err := t.deleteNode(t.root, key)
	return err
}

func (t *RadixTrie) Proof(key []byte) ([][]byte, error) {
	node, err := t.findNode(key)
	if err != nil {
		return nil, err
	}
	proofs := [][]byte{}
	currentNode := t.root
	for _, k := range key {
		if currentNode == nil {
			return nil, errors.New("key not found")
		}
		for childKey, childNode := range currentNode.children {
			if childKey != k {
				proof, _ := encodeNode(childNode)
				proofs = append(proofs, proof)
			}
		}
		currentNode = currentNode.children[k]
	}
	valueProof, _ := encodeNode(node)
	proofs = append(proofs, valueProof)
	return proofs, nil
}

// Helper functions

func (t *RadixTrie) findNode(key []byte) (*Node, error) {
	currentNode := t.root
	for _, k := range key {
		if currentNode == nil {
			return nil, errors.New("key not found")
		}
		currentNode = currentNode.children[k]
	}
	return currentNode, nil
}

func (t *RadixTrie) insertNode(node *Node, key, value []byte) *Node {
	if node == nil {
		node = &Node{
			children: make(map[byte]*Node),
		}
	}
	if len(key) == 0 {
		node.value = value
		return node
	}
	firstChar := key[0]
	node.children[firstChar] = t.insertNode(node.children[firstChar], key[1:], value)
	return node
}

func (t *RadixTrie) deleteNode(node *Node, key []byte) (*Node, error) {
	if node == nil {
		return nil, errors.New("key not found")
	}
	if len(key) == 0 {
		if node.value == nil {
			return nil, errors.New("key not found")
		}
		node.value = nil
		if len(node.children) == 0 {
			return nil, nil
		}
		return node, nil
	}
	firstChar := key[0]
	childNode, err := t.deleteNode(node.children[firstChar], key[1:])
	if err != nil {
		return nil, err
	}
	if childNode == nil {
		delete(node.children, firstChar)
		if len(node.children) == 0 && node.value == nil {
			return nil, nil
		}
	}
	return node, nil
}

func encodeNode(node *Node) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	err := encoder.Encode(node)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func hash(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}

func main() {
	// Example usage
	db := NewDatabase()
	trie := &RadixTrie{
		db:         db,
		dirtyNodes: make(map[string]*Node),
	}

	// Seed random number generator
	rand.Seed(time.Now().UnixNano())

	ticker := time.NewTicker(2 * time.Second)
	keys := [][]byte{}
	go func() {
		i := 1
		for range ticker.C {
			key := []byte(fmt.Sprintf("key%d", i))
			value := []byte(fmt.Sprintf("value%d", i))
			trie.Put(key, value)
			trie.Commit()
			fetchedValue, err := trie.Get(key)
			if err != nil {
				fmt.Println("Error:", err)
			} else {
				fmt.Println("Fetched Value:", string(fetchedValue))
			}
			keys = append(keys, key)
			i++
		}
	}()

	proofTicker := time.NewTicker(5 * time.Second)
	go func() {
		for range proofTicker.C {
			if len(keys) == 0 {
				continue
			}
			randomIndex := rand.Intn(len(keys) + 3) // Include some undefined keys
			var randomKey []byte
			if randomIndex < len(keys) {
				randomKey = keys[randomIndex]
			} else {
				randomKey = []byte(fmt.Sprintf("undefined_key%d", randomIndex-len(keys)))
			}
			proofs, err := trie.Proof(randomKey)
			if err != nil {
				fmt.Printf("Error for key %s: %s\n", randomKey, err)
			} else {
				fmt.Printf("Random Proof for key %s:\n", randomKey)
				for i, proof := range proofs {
					fmt.Printf("Proof %d: %x\n", i, hash(proof))
				}
			}
		}
	}()

	// Keep the program running to see the ticker in action
	select {}
}
