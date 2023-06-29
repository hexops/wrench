package zon

import (
	"fmt"
	"io"
)

func Parse(contents string) (*Node, error) {
	var (
		nextState = "start"
		line      = 0
		column    = 0
		tree      *Node
		stack     []*Node
		stackName []string
		tagName   string
		stringLit string
	)
	for _, c := range contents {
		column++
		if c == '\n' {
			line++
			column = 0
		}
		if c == ' ' || c == '\n' {
			continue
		}
		expect := func(expected rune, state string) error {
			if c != expected {
				return fmt.Errorf("%v:%v: expected %s ('%s'), found %s", line, column, state, string(expected), string(c))
			}
			return nil
		}
		// fmt.Printf("%v:%v: %s %s - %s\n", line, column, string(c), nextState, stackName)
		switch nextState {
		case "start":
			if err := expect('.', nextState); err != nil {
				return nil, err
			}
			nextState = "tag or object"
		case "tag or object":
			if c == '{' {
				if err := expect('{', nextState); err != nil {
					return nil, err
				}
				nextState = "value"
				if tree == nil {
					stack = append(stack, &Node{})
					stackName = append(stackName, "root")
					tree = stack[len(stack)-1]
				}
			} else {
				tagName += string(c)
				nextState = "tag"
			}
		case "value":
			if c == '"' {
				if err := expect('"', nextState); err != nil {
					return nil, err
				}
				nextState = "string literal"
			} else if c == '}' {
				// object close
				stack = stack[:len(stack)-1]
				stackName = stackName[:len(stackName)-1]
				nextState = "next value"
			} else {
				if err := expect('.', nextState); err != nil {
					return nil, err
				}
				nextState = "tag or object"
			}
		case "next value":
			if c == '}' {
				// object close
				stack = stack[:len(stack)-1]
				stackName = stackName[:len(stackName)-1]
				nextState = "next value"
				continue
			}
			if err := expect(',', nextState); err != nil {
				return nil, err
			}
			nextState = "value"
		case "tag":
			if c == '=' {
				parent := stack[len(stack)-1]
				parent.Tags = append(parent.Tags, Tag{Name: tagName, Node: Node{}})
				stack = append(stack, &parent.Tags[len(parent.Tags)-1].Node)
				stackName = append(stackName, tagName)
				nextState = "value"
				tagName = ""
				continue
			} else {
				tagName += string(c)
			}
		case "string literal":
			if c == '"' {
				stack[len(stack)-1].StringLiteral = stringLit
				stack = stack[:len(stack)-1]
				stackName = stackName[:len(stackName)-1]
				tagName = ""
				stringLit = ""
				nextState = "next value"
			} else {
				stringLit += string(c)
			}
		}
	}
	if len(stack) != 0 {
		fmt.Println(len(stack), stackName)
		panic("unexpected: stack not emptied")
	}
	if tree == nil {
		return &Node{}, nil
	}
	return tree, nil
}

type Tag struct {
	Name string
	Node Node
}

type Node struct {
	Tags          []Tag
	StringLiteral string
}

func (n *Node) Write(w io.Writer, indent, prefix string) error {
	if n.StringLiteral != "" {
		fmt.Fprintf(w, "%q", n.StringLiteral)
		return nil
	}
	fmt.Fprintf(w, ".{\n")
	for _, tag := range n.Tags {
		fmt.Fprintf(w, prefix+indent+".%s = ", tag.Name)
		_ = tag.Node.Write(w, indent, prefix+indent)
		fmt.Fprintf(w, ",")
		fmt.Fprintf(w, "\n")
	}
	fmt.Fprintf(w, prefix+"}")
	return nil
}

func (n *Node) Child(tagName string) *Node {
	for i, tag := range n.Tags {
		if tag.Name == tagName {
			return &n.Tags[i].Node
		}
	}
	return nil
}
