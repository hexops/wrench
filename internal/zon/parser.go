package zon

import (
	"fmt"
	"io"
)

const (
	stateStart              = "start"
	stateDot                = "dot"
	stateValue              = "value"
	stateValueComplete      = "value-complete"
	stateNextValue          = "next-value"
	stateDotName            = "dot-name"
	stateStartStringLiteral = "start-string-literal"
	stateStringLiteral      = "string-literal"
	stateStartComment       = "start-comment"
	stateComment            = "comment"
)

func Parse(contents string) (*Node, error) {
	var (
		nextState = stateStart
		prevState = stateStart
		line      = 0
		column    = 0
		stack     []*Node
		stackName []string
		tagName   string
		runes     = []rune(contents)
	)
	stackPop := func() {
		stack = stack[:len(stack)-1]
		stackName = stackName[:len(stackName)-1]
	}
	stackPush := func(n *Node, name string) {
		stack = append(stack, n)
		stackName = append(stackName, name)
	}
	root := &Node{Root: true}
	stackPush(root, "root")
	for i := 0; i < len(runes); i++ {
		i--
		var c rune
		next := func() {
			i++
			c = runes[i]
			column++
			if c == '\n' {
				line++
				column = 0
			}
		}
		next()
		prev := func() {
			if c == '\n' {
				line--
			}
			i--
			c = runes[i]
			column--
		}
		expect := func(expected rune, state string) error {
			if c != expected {
				return fmt.Errorf("%v:%v: expected %s (%q), found %q", line, column, state, string(expected), string(c))
			}
			return nil
		}
		// fmt.Printf("%v:%v: %q %s - %s\n", line, column, string(c), nextState, stackName)
		switch nextState {
		case stateStart:
			if c == ' ' || c == '\n' {
			} else if c == '/' {
				prevState = stateStart
				nextState = stateStartComment
			} else {
				if err := expect('.', nextState); err != nil {
					return nil, err
				}
				nextState = stateDot
			}
		case stateStartComment:
			if err := expect('/', nextState); err != nil {
				return nil, err
			}
			parent := stack[len(stack)-1]
			commentNode := &Node{Comment: "//"}
			parent.Children = append(parent.Children, commentNode)
			stackPush(commentNode, "comment")
			nextState = stateComment
		case stateComment:
			commentNode := stack[len(stack)-1]
			commentNode.Comment += string(c)
			if c == '\n' {
				stackPop()
				nextState = prevState
			}
		case stateDot:
			if c == '{' {
				if err := expect('{', nextState); err != nil {
					return nil, err
				}
				anonStruct := &Node{}
				stackPush(anonStruct, "anon-struct")
				nextState = stateValue
			} else {
				tagName += string(c)
				nextState = stateDotName
			}
		case stateValue:
			if c == ' ' || c == '\n' {
			} else if c == '"' {
				prev()
				nextState = stateStartStringLiteral
			} else if c == '}' {
				nextState = stateValueComplete
			} else {
				if err := expect('.', nextState); err != nil {
					return nil, err
				}
				nextState = stateDot
			}
		case stateValueComplete:
			prev()
			complete := stack[len(stack)-1]
			stackPop()
			parent := stack[len(stack)-1]
			if parent.DotName != "" {
				parent.DotValue = complete
				stackPop()
			} else {
				parent.Children = append(parent.Children, complete)
			}
			nextState = stateNextValue
		case stateNextValue:
			if c == ' ' || c == '\n' {
			} else if c == '}' {
				nextState = stateValueComplete
			} else {
				if err := expect(',', nextState); err != nil {
					return nil, err
				}
				nextState = stateValue
			}
		case stateDotName:
			if c == ' ' || c == '\n' {
			} else if c == '=' {
				parent := stack[len(stack)-1]
				dotValueNode := &Node{DotName: tagName}
				parent.Children = append(parent.Children, dotValueNode)
				stackPush(dotValueNode, "."+tagName)
				nextState = stateValue
				tagName = ""
				continue
			} else {
				tagName += string(c)
			}

		case stateStartStringLiteral:
			if err := expect('"', nextState); err != nil {
				return nil, err
			}
			parent := stack[len(stack)-1]
			stringNode := &Node{StringLiteral: ""}
			parent.Children = append(parent.Children, stringNode)
			stackPush(stringNode, "string-literal")
			nextState = stateStringLiteral
		case stateStringLiteral:
			stringNode := stack[len(stack)-1]
			if c == '"' {
				nextState = stateValueComplete
			} else {
				stringNode.StringLiteral += string(c)
			}
		}
	}
	if len(stack) != 1 {
		fmt.Println(len(stack), stackName)
		return nil, fmt.Errorf("stack not emptied to just [root], assignment '=' without value?")
	}
	stackPop()
	return root, nil
}

type Node struct {
	Root          bool
	DotName       string
	DotValue      *Node
	StringLiteral string
	Comment       string
	Children      []*Node
}

func (n *Node) Write(w io.Writer, indent, prefix string) error {
	if err := n.write(w, indent, prefix); err != nil {
		return err
	}
	fmt.Fprintf(w, "\n")
	return nil
}

func (n *Node) write(w io.Writer, indent, prefix string) error {
	if n.DotName != "" {
		fmt.Fprintf(w, ".%s = ", n.DotName)
		_ = n.DotValue.write(w, indent, prefix)
		return nil
	} else if n.StringLiteral != "" {
		fmt.Fprintf(w, "%q", n.StringLiteral)
		return nil
	} else if n.Comment != "" {
		fmt.Fprintf(w, "%s", n.Comment)
		return nil
	} else if n.Root && len(n.Children) > 0 {
		for _, child := range n.Children {
			_ = child.write(w, indent, prefix)
		}
		return nil
	}
	if len(n.Children) > 0 {
		fmt.Fprintf(w, ".{\n")
	} else {
		fmt.Fprintf(w, ".{")
	}
	for _, child := range n.Children {
		fmt.Fprintf(w, prefix+indent)
		_ = child.write(w, indent, prefix+indent)
		fmt.Fprintf(w, ",\n")
	}
	fmt.Fprintf(w, prefix+"}")
	return nil
}

// func (n *Node) Child(tagName string) *Node {
// 	for i, tag := range n.Tags {
// 		if tag.Name == tagName {
// 			return &n.Tags[i].Node
// 		}
// 	}
// 	return nil
// }
