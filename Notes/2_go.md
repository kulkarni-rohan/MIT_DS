# Golang
## Go basics
### I. packages, variables and functions
1. a name is exported if it begins with a capital letter
2. named return values, treated as initialized at the top of the function
3. short variable declarations, implicit type
```
c, python, java := true, false, "no!"
```
4. basic types
```
bool
string
int  int8  int16  int32  int64
uint uint8 uint16 uint32 uint64 uintptr
byte // alias for uint8
rune // alias for int32
     // represents a Unicode code point
float32 float64
complex64 complex128
```
5. type conversions: T(v)
```
i := 42
f := float64(i)
u := uint(f)
```
6. constants
```
const World = "世界"
```
### II. flow control
1. for, init and post statements are optional
```
for i := 0; i < 10; i++ {
    sum += i
}
```
for is go's "while"
```
for sum < 1000 {
    sum += sum
}
```
forever
```
for {
}
```
2. if can start with a short statement, in scope until the end of if
```
if v := math.Pow(x, n); v < lim {
    return v
}
```
3. Switch without a condition is the same as switch true
4. `defer` function calls are pushed onto a stack. When a function returns, its deferred calls are executed in last-in-first-out order.
### III. more types: structs slices and maps
1. pointers same as c except that Go has no pointer arithmetic
2. struct, accessed using dot, pointer to struct also dot
```
type Vertex struct {
	X int
	Y int
}
var (
	v2 = Vertex{X: 1}  // Y:0 is implicit
	v3 = Vertex{}      // X:0 and Y:0
)
```
3. arrays, Slices are like references to arrays
```
var a [10]int
a[low : high]
```
4. slice literals\
array literal:
```
[3]bool{true, true, false}
```
this creates the same array as above, then builds a slice that references it:
```
[]bool{true, true, false}
```
5. slice defaults
```
a[0:10]
a[:10]
a[0:]
a[:]
```
6. creating a slice with make
```
a := make([]int, 5)  // len(a)=5
b := make([]int, 0, 5) // len(b)=0, cap(b)=5
b = b[:cap(b)] // len(b)=5, cap(b)=5
b = b[1:]      // len(b)=4, cap(b)=4
```
append
```
s = append(s, 0)
```
7. range
```
for i, v := range pow {
    fmt.Printf("2**%d = %d\n", i, v)
}
```
skip the index or value by assigning to _\
If you only want the index, you can omit the second variable.
8. maps\
The make function returns a map of the given type, initialized and ready for use.
```
var m map[string]Vertex
func main() {
	m = make(map[string]Vertex)
}
```
map literals
```
var m = map[string]Vertex{
	"Bell Labs": Vertex{
		40.68433, -74.39967,
	},
	"Google": Vertex{
		37.42202, -122.08408,
	},
}

var m = map[string]Vertex{
	"Bell Labs": {40.68433, -74.39967},
	"Google":    {37.42202, -122.08408},
}
```
9. mutating maps\
Insert or update an element in map m:
```
m[key] = elem
```
Retrieve an element:
```
elem = m[key]
```
Delete an element:
```
delete(m, key)
```
Test that a key is present with a two-value assignment:
```
elem, ok = m[key]
```
10. function closures\
A closure is a function value that references variables from outside its body.
```
func adder() func(int) int {
	sum := 0
	return func(x int) int {
		sum += x
		return sum
	}
}
```
## Methods and Interfaces
### I. Methods
go does not have classes. However, you can define methods on types. receiver appears in its own argument list between the func keyword and the method name.
```
func (v Vertex) Abs() float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y)
}
```
You can only declare a method with a receiver whose type is defined in the same package as the method. You cannot declare a method with a receiver whose type is defined in another package (which includes the built-in types such as int).
### II. pointer receivers
1. Methods with pointer receivers can modify the value to which the receiver points
2. functions with a pointer argument must take a pointer:
3. methods with pointer receivers take either a value or a pointer as the receiver.
### III. interfaces
1. An interface type is defined as a set of method signatures.
2. Interfaces are implemented implicitly
3. Under the hood, interface values can be thought of as a tuple of a value and a concrete type
4. If the concrete value inside the interface itself is nil, the method will be called with a nil receiver.
5. A nil interface value holds neither value nor concrete type.
6. The interface type that specifies zero methods is known as the empty interface, can hold values of any type. 
7. type assertions
```
var i interface{} = "hello"

s := i.(string)
fmt.Println(s)

s, ok := i.(string)
fmt.Println(s, ok)

f, ok := i.(float64)
fmt.Println(f, ok)

f = i.(float64) // panic
fmt.Println(f)
```
8. type switches
```
switch v := i.(type) {
case T:
    // here v has type T
case S:
    // here v has type S
default:
    // no match; here v has the same type as i
}
```
9. Stringer, A Stringer is a type that can describe itself as a string. The fmt package (and many others) look for this interface to print values.
### IV. Errors 
The error type is a built-in interface similar to fmt.Stringer
```
type error interface {
    Error() string
}
```
```
i, err := strconv.Atoi("42")
```
### V. Readers
```
r := strings.NewReader("Hello, Reader!")

b := make([]byte, 8)
for {
    n, err := r.Read(b)
    fmt.Printf("n = %v err = %v b = %v\n", n, err, b)
    fmt.Printf("b[:n] = %q\n", b[:n])
    if err == io.EOF {
        break
    }
}
```
### VI. images
```
package image

type Image interface {
    ColorModel() color.Model
    Bounds() Rectangle
    At(x, y int) color.Color
}
```
## Concurrency
### I. Goroutines
```
go f(x, y, z)
```
### II. Channels
```
ch <- v    // Send v to channel ch.
v := <-ch  // Receive from ch, and
           // assign value to v.
```
create channel
```
ch := make(chan int)
```
sends and receives block until the other side is ready. This allows goroutines to synchronize without explicit locks or condition variables.
### III. Buffered Channels
```
ch := make(chan int, 100)
```
### IV. Range and Close
1. The loop for i := range c receives values from the channel repeatedly until it is closed.
2. `v, ok := <-ch`
### V. Select
A select blocks until one of its cases can run, then it executes that case. It chooses one at random if multiple are ready.
### VI. sync.Mutex
```
mu sync.Mutex
c.mu.Lock()
c.mu.Unlock()
```