# Concurrency in Goja: A Technical Guide for Experienced Engineers

## Chapter 1: Introduction

Goja is a JavaScript runtime implemented in pure Go, enabling Go applications to execute JavaScript code. Concurrency in Goja operates differently from Go’s native goroutine model – it follows JavaScript’s single-threaded event loop concurrency paradigm. This book provides a comprehensive look at how Goja handles concurrency and how you, as a senior engineer, can effectively integrate Go’s concurrent features with Goja’s single-threaded runtime. We will explore core concepts such as the simulated JavaScript event loop in Goja, the use of Promises and async/await (including their current support and limitations), and patterns for safely bridging Go goroutines with Goja’s event-driven model. Advanced topics like performance tuning, maintaining state safety across concurrency boundaries, and error propagation between Go and JavaScript are addressed in depth. By the end, you will have practical knowledge of writing concurrent code with Goja, integrating native Go modules, exposing asynchronous APIs to JavaScript, and best practices for designing robust, concurrent systems using Goja.

**What to Expect:** Each chapter delves into specific aspects of Goja’s concurrency model. We start with an overview of JavaScript’s concurrency model and how Goja simulates an event loop. Then we discuss Goja’s support for asynchronous programming constructs (Promises and `async/await`), and how to use the `goja_nodejs/eventloop` package to implement timers, I/O, and task scheduling. We will demonstrate how to safely call Goja from multiple goroutines and how to return control back to JavaScript’s single thread when background tasks complete. Practical code examples and patterns are included throughout to illustrate key techniques, along with exercises and project ideas to solidify your understanding. Finally, a chapter on best practices and design patterns summarizes recommendations for building high-performance, safe concurrent systems with Goja.

Before diving in, it’s important to note that while Goja allows embedding JavaScript into Go programs, it **is not thread-safe to use a single Goja runtime from multiple goroutines simultaneously**. Instead, concurrency is achieved by scheduling tasks on a single-threaded event loop or by using multiple separate Goja runtime instances in parallel. With this in mind, let’s begin by reviewing the fundamental concurrency model that JavaScript uses and how Goja adopts and simulates this model.

## Chapter 2: JavaScript Concurrency Model and Goja’s Single-Threaded Execution

JavaScript’s concurrency model is based on an **event loop** and a **single-threaded execution** environment. In a browser or Node.js, the JavaScript engine runs on one thread, and asynchronous operations (like timers, I/O events, or Promise completions) are handled by queuing callbacks to be executed by the event loop. This means JavaScript can perform non-blocking operations and appear to handle many tasks “at once,” but under the hood it is interleaving tasks on one thread via the event loop. In Node.js, for example, I/O operations are offloaded to the system or worker threads, but their callbacks are queued back on the main thread’s event loop for execution. The key point is that JavaScript achieves concurrency through an event-driven, asynchronous model rather than true parallel threads.

**Goja’s Single-Threaded Runtime:** Goja follows this same model. A Goja `Runtime` instance executes JavaScript code on a single goroutine (thread) and does not intrinsically support multi-threaded execution of JavaScript code. In fact, the Goja documentation explicitly states that a `goja.Runtime` is *not* goroutine-safe – it can only be used by one goroutine at a time. You can create multiple Goja runtimes (for example, one per OS thread or one per task) to run scripts concurrently, but each runtime instance must be confined to a single thread of execution. It’s not possible to directly share complex JavaScript objects between different runtime instances either, so each acts as an isolated world (similar to separate browser windows or Node.js worker threads, conceptually).

**Event Loop Concurrency in Goja:** Because Goja itself doesn’t include a built-in event loop or scheduler, the **hosting Go application is responsible for simulating the event loop** if needed. In practice, this means that concurrent behavior (timers, async callbacks, etc.) must be orchestrated by Go code that integrates with the Goja runtime. Goja executes JavaScript code to completion (e.g. when you call `vm.RunString` it will run all the code in that call synchronously until it returns or throws). Any asynchronous JavaScript concept like `setTimeout`, `Promise` resolution, or `async/await` needs an external mechanism to *schedule* the continuation of execution. We will see how the `goja_nodejs` library provides this mechanism via an event loop abstraction.

**Summary:** In Goja, as in JavaScript generally, concurrency is achieved by **non-blocking operations and an event loop** rather than by multi-threading within a single runtime. A single Goja runtime behaves like a JavaScript engine running on one thread that can queue up tasks to be executed one after another. Understanding this single-threaded, event-driven model is crucial for safely integrating Go’s concurrency (goroutines) with Goja, which we will explore in later chapters. Next, we will discuss how to simulate and manage an event loop in Goja to enable JavaScript’s asynchronous features like timers and deferred callbacks.

## Chapter 3: Simulating the JavaScript Event Loop in Goja

In a browser or Node.js, the event loop is what allows JavaScript to handle asynchronous events (timers, I/O completions, UI events, etc.) without blocking the single thread. For Goja, the **`goja_nodejs/eventloop`** package provides a ready-made event loop simulation that mimics Node.js style asynchronous behavior. Using this event loop, we can schedule JavaScript functions to run after a delay, at intervals, or when external asynchronous operations complete, just as we would with `setTimeout`, `setInterval`, or event callbacks in a normal JS environment.

### 3.1 The `goja_nodejs/eventloop` Package Overview

The `eventloop` package offers an `EventLoop` type, which essentially runs a loop in a separate goroutine, executing tasks on a Goja `Runtime` one at a time. Key methods provided by `EventLoop` include:

* **`Start()` / `StartInForeground()`** – to start the event loop. `Start()` runs the loop in a new background goroutine, whereas `StartInForeground()` runs it in the current goroutine (allowing you to catch panics from tasks, which can aid in debugging).
* **`Stop()` / `Terminate()`** – to stop the loop. `Stop()` will let the loop finish processing pending tasks before stopping, while `Terminate()` stops immediately and clears any scheduled timers.
* **Task scheduling methods:**

  * `Run(fn)` – run a function on the loop immediately and **wait** for it to finish, then stop the loop when no more tasks remain. This is useful for running a whole script with an event loop in one go.
  * `RunOnLoop(fn)` – schedule a function to run on the loop as soon as possible (returns immediately to the caller, and the task executes on the loop thread).
  * `SetTimeout(fn, duration)` – schedule a function to run after a specified timeout (analogous to `setTimeout` in JavaScript).
  * `SetInterval(fn, duration)` – schedule a function to run periodically at the given interval (analogous to `setInterval`).
  * Additionally, `ClearTimeout` and `ClearInterval` to cancel timers if needed.

Under the hood, the event loop uses a single Goja `Runtime` instance. When you create a new `EventLoop` via `NewEventLoop()`, it internally creates a Goja runtime (with some optional configurations, like enabling a console module by default). All tasks scheduled on this loop will be executed on that one JavaScript runtime sequentially. This means you can treat it similarly to Node.js’s main thread: schedule many asynchronous operations, and they won’t overlap or race with each other on the JavaScript side.

**Example – Basic Event Loop Usage:** Below is a simple example of using the event loop to simulate a timer in Goja:

```go
import (
    "time"
    "github.com/dop251/goja_nodejs/eventloop"
)

loop := eventloop.NewEventLoop()  // create a new event loop and JS runtime
loop.Start()                      // start the loop in background goroutine
defer loop.Stop()                 // ensure we stop the loop when done

// Schedule a function to run in 1 second (1000ms), like setTimeout
loop.SetTimeout(func(vm *goja.Runtime) {
    vm.RunString(`console.log("Timeout triggered at " + Date.now())`)
}, 1*time.Second)

// You can do other work here in Go while the JS timer is waiting...

// Optionally, block until the loop has no more pending tasks:
loop.RunOnLoop(func(vm *goja.Runtime) {
    // This runs immediately on the loop.
    // After this, since no recurring tasks remain beyond the SetTimeout, 
    // the loop will eventually become idle.
})
time.Sleep(2 * time.Second)
```

In this snippet, we create and start an `EventLoop`, schedule a JavaScript `console.log` to execute after 1 second, and then let the program sleep for 2 seconds to ensure the timer fires. The `console.log` output would occur from within the Goja runtime’s console (which is enabled by default in the loop) and would print a timestamp message after one second. This mimics `setTimeout` behavior.

A few important points demonstrated here:

* We call `loop.Start()` to begin processing tasks. Always start the loop (and consider deferring `Stop()` or `Terminate()` to clean up) before scheduling tasks.
* We used `SetTimeout` with a Go function that invokes JavaScript (via `vm.RunString` in this case) to log a message. The `vm *goja.Runtime` passed into the function is the event loop’s JavaScript runtime – you should use this for any JS interactions inside the scheduled function.
* We could similarly use `loop.SetInterval` to schedule a repeating task. That returns an `Interval` handle which we could later cancel with `loop.ClearInterval` if needed.
* We used `RunOnLoop` with an empty function to illustrate scheduling an immediate task. In practice, `RunOnLoop` is extremely useful for interacting with the loop from other goroutines, as we’ll see in the next chapter.

The `EventLoop` ensures that only one task runs at a time on its JavaScript runtime, preserving thread safety. You can schedule tasks from any goroutine – `RunOnLoop`, `SetTimeout`, and `SetInterval` are safe to call concurrently (they internally synchronize as needed). This is how we bridge Go’s concurrency to the single-threaded JS concurrency: by *posting* work to the JS thread’s queue.

### 3.2 Simulating Timers, I/O, and Async Tasks

**Timers:** As shown, `SetTimeout` and `SetInterval` allow us to schedule JavaScript code to run later or repeatedly. For example, to simulate `setInterval` that prints a heartbeat message every second, you could do:

```go
timer := loop.SetInterval(func(vm *goja.Runtime) {
    vm.RunString(`console.log("Heartbeat - " + Date.now())`)
}, 1*time.Second)
// ... later, if we want to stop:
loop.ClearInterval(timer)
```

This would call the provided function every second on the loop’s thread, which in turn executes a JS console log. The design is analogous to using `setInterval` in Node.js, including the ability to cancel via `ClearInterval`.

**Asynchronous I/O and Tasks:** The event loop by itself doesn’t perform I/O – that’s up to you. However, you can integrate I/O by performing it in Go (outside the JS runtime) and then scheduling a callback in JS when it’s done. Typically, you might use the **Promise pattern** or Node-style callbacks for this. We’ll cover Promises in the next chapter, but here’s a brief look at using `RunOnLoop` to handle an asynchronous task:

Suppose we want to expose a function in JS that reads a file and then calls a JS callback when done. In Go, you could do:

```go
// In the initialization, expose a global function "readFile" to JS:
vm := loop.Runtime()  // assume we have access to the loop's runtime
vm.Set("readFile", func(call goja.FunctionCall) goja.Value {
    pathVal := call.Argument(0)
    cbVal := call.Argument(1)
    if cbVal.ExportType().Kind() != reflect.Func {
        panic(vm.ToValue("Callback not provided"))
    }
    path := pathVal.String()
    cb := goja.AssertFunction(cbVal)  // get a Callable for the JS callback

    // Perform file read in a separate goroutine to avoid blocking the event loop:
    go func() {
        data, err := ioutil.ReadFile(path)
        loop.RunOnLoop(func(vm *goja.Runtime) {
            // Now on JS thread, call the callback with (err, data)
            if err != nil {
                cb(goja.Undefined(), vm.ToValue(nil), vm.ToValue(err.Error()))
            } else {
                cb(goja.Undefined(), vm.ToValue(string(data)), goja.Null()) 
                // convention: pass null for error if none
            }
        })
    }()
    return goja.Undefined()  // readFile returns undefined (results delivered via callback)
})
```

In this pseudo-code, we did the following:

* Defined a Go function `readFile(path, callback)` accessible in JS. It takes a file path and a JavaScript callback function.
* Inside `readFile`, we launch a new goroutine to read the file using Go’s `ioutil.ReadFile`. This is the long-running or blocking I/O operation, done outside the JS thread.
* Once the file read completes (or fails), we call `loop.RunOnLoop` with a function that invokes the original JS callback on the Goja runtime. We must do this because calling the JS callback (`cb(...)`) is a manipulation of the JS state, which **must happen on the event loop’s thread** (the JS thread). We pass the file data or error message to the callback.
* `readFile` returns immediately (undefined in JS), letting the JS event loop continue. The actual callback will be executed later when the goroutine schedules it via `RunOnLoop`.

This pattern – doing work in Go concurrently, then scheduling a JS callback via the event loop – is a common way to integrate asynchronous tasks. It ensures that heavy I/O or computation doesn’t block the JS execution, while preserving thread safety by funneling the callback invocation back through the single-threaded loop.

**Important Constraints:** The example above highlights a critical rule: any interaction with the Goja `Runtime` (such as calling a JS function, creating new JS values, or modifying JS objects) must happen on that runtime’s own thread. The `EventLoop` gives us the `RunOnLoop` method to safely enqueue such interactions. If you attempt to call a JS function from a background goroutine without `RunOnLoop`, you will encounter panics or memory errors. In fact, as the Goja author notes, this is expected behavior – calling into the VM from another goroutine will corrupt the VM state or crash the program. Therefore, always use the event loop’s scheduling functions or other synchronization (like channels or mutexes) to ensure only one goroutine is manipulating a Goja runtime at a time.

Now that we have a functioning event loop and an understanding of how to queue tasks on it, we can proceed to explore JavaScript’s native async abstractions – **Promises and async/await** – and how they work (and sometimes need help) in Goja.

## Chapter 4: Promises and Async/Await in Goja – Support and Limitations

Promises are the modern standard for asynchronous programming in JavaScript, and `async/await` syntax builds on Promises to allow writing asynchronous code in a synchronous style. Goja introduced support for Promises and even the `async/await` syntax, but there are some nuances and limitations to be aware of when using them in the Goja environment.

### 4.1 Promise Support in Goja

Goja fully supports the creation and usage of JavaScript Promise objects. You can create a promise either from within JavaScript (e.g. using `new Promise((resolve, reject) => { ... })` in a script) or via the Goja API. The Goja `Runtime` provides a method `NewPromise()` which can be used from Go code to create a pending Promise along with its `resolve` and `reject` functions. This is extremely useful for bridging between Go and JS – for example, to return a Promise from a Go-provided function.

However, using `NewPromise()` correctly requires the event loop integration we discussed. The Goja documentation warns that the resolve/reject functions returned by `NewPromise()` are *not goroutine-safe* and must be called only when the VM is in a safe state (i.e., on the VM’s thread). In practice, this means if you create a Promise in a Go function, you should schedule any call to `resolve` or `reject` through `RunOnLoop` (or ensure it’s on the same thread that’s currently running the VM code).

**Example – Using `NewPromise` in an Event Loop:** Let’s say we want to expose a JS function that returns a Promise which resolves after a delay (like a sleep function). We can implement it in Go as:

```go
vm.Set("sleep", func(call goja.FunctionCall) goja.Value {
    delay := time.Duration(call.Argument(0).ToInteger()) * time.Millisecond
    // Create a new Promise and get its resolve function
    promise, resolve, reject := vm.NewPromise()
    // Launch a goroutine to wait and then resolve
    go func() {
        time.Sleep(delay)
        loop.RunOnLoop(func(vm *goja.Runtime) {
            // Resolve the promise on the JS thread
            err := resolve(goja.Undefined()) 
            if err != nil {
                // Handle uncatchable errors (InterruptedError, etc.)
                fmt.Println("Failed to resolve promise:", err)
            }
        })
    }()
    return promise // return the Promise object to JavaScript
})
```

If a JS script calls `sleep(500)`, it will immediately get back a Promise, and 500ms later that Promise will be resolved (with an undefined value in this case). The actual sleeping happened in a goroutine, and then we invoked the `resolve` function via `loop.RunOnLoop`. This pattern ensures thread safety – calling `resolve` directly inside the goroutine would violate the rules and likely panic the runtime.

**Promise Microtask Queue:** In JavaScript, Promise resolutions don’t execute their `.then()` callbacks immediately; they are queued as microtasks to run after the current script turn completes. Goja implements this behavior internally. When you resolve a Promise (either from JS or via `resolve()` in Go), the attached callbacks (or the continuation of an `await`) are scheduled to run. If you are running Goja with the event loop (via `EventLoop.Run` or continuously with `Start()`), the microtasks (Promise reaction jobs) will be executed at the appropriate time by the engine. Typically, Goja will run any pending Promise jobs when the call stack is unwound, similar to how a browser would between tasks. If you use `EventLoop.Run(fn)`, it will actually run until the event loop has no more tasks *and* all Promise jobs are processed before returning. This means that using the event loop is usually sufficient to ensure promises resolve and their callbacks execute.

For example, if in a script you do:

```javascript
let p = new Promise(resolve => resolve(42));
p.then(val => console.log("Promise resolved with", val));
```

The `.then` callback will be executed as a microtask after the current execution finishes. If you are inside an `EventLoop.Run`, it will happen before `Run` returns. If you’re in an interactive or long-running event loop (`Start()`), it will happen very shortly after scheduling (essentially immediately after the current operation yields back to the loop).

### 4.2 Async/Await Syntax in Goja

Originally, Goja did not support `async`/`await` at all (since those came after ES5). However, as of late 2022, the Goja project implemented support for the `async/await` syntax and semantics. This means you can define async functions in Goja and use `await` within them. The engine will “pause” the function at the await point and resume it when the awaited Promise resolves, just like in a standard JavaScript engine.

**Example:**

```javascript
// In a Goja script context
async function fetchData() {
    let result = await sleep(1000);  // using the sleep() promise from earlier
    console.log("Fetched data:", result);
}
fetchData();
```

In this snippet, calling `fetchData()` will immediately return a Promise (because an async function always returns a Promise). The function’s execution will suspend at the `await sleep(1000)` line. After 1 second, when the `sleep(1000)` Promise resolves, Goja will automatically resume the `fetchData` function and execute the `console.log`. All of this happens on the single thread of the event loop.

One limitation to note: **top-level await** (using `await` at the top level of a script without wrapping in an async function) is not supported in Goja, because it’s part of ES modules, which Goja doesn’t fully implement. You should always use `await` inside an `async function` in Goja. Also, generator functions and `async generators` are not supported as of writing, which means no `for await` loops over async iterators, etc., in Goja.

**Async/Await and the Event Loop:** Even though async/await makes code look synchronous, under the hood it still relies on the promise mechanism. Therefore, using `await` inherently requires that promise callbacks be processed – which again implies the event loop or an equivalent mechanism. If you call an async function and immediately wait on the returned Promise from Go (for example, by exporting it and calling from Go code), you need to ensure the event loop runs to drive the await. If you use `EventLoop.Run` to run a script that includes an `await`, `Run` will block until the async function completes (because `Run` will keep the loop alive until no more tasks or microtasks remain).

Internally, Goja’s implementation of `async/await` uses the concept of an **async context** for each async function invocation. Goja provides an `AsyncContextTracker` interface that advanced users can use to track async call contexts across promise continuations. This is similar in spirit to Node’s AsyncLocalStorage (to track context like transaction IDs across awaits). The AsyncContextTracker isn’t required for normal usage, but it’s good to know it exists: whenever an async function is paused at an await and later resumed, the tracker (if set) ensures you can restore any associated context. This works for both async/await and plain Promise `.then()` callbacks.

### 4.3 Limitations and Common Pitfalls with Promises

**Unhandled Promise Rejections:** In Node.js, if a Promise is rejected and no `.catch` or error handler is attached, it might emit a warning or even crash (depending on settings). In Goja, an unhandled promise rejection will not by default throw an error in Go – it will just remain unhandled. However, Goja provides a hook to catch these: `SetPromiseRejectionTracker` on the Runtime allows you to register a function to be called when a Promise is rejected without a handler, and when such a rejected promise later gains a handler. Using this, you can log warnings or decide to `panic` if a promise rejection was unhandled (to mimic Node’s behavior). For example, you might do:

```go
vm.SetPromiseRejectionTracker(func(p *goja.Promise, operation goja.PromiseRejectionOperation) {
    if operation == goja.PromiseRejectionReject {
        fmt.Printf("Unhandled promise rejection: %v\n", p.Result().Export())
    }
})
```

This would print a message whenever a promise is rejected without any catch at the time of rejection. It’s a good practice to use this in development to catch forgotten `.catch` handlers.

**Long-Running or Blocking Operations in Promises:** A common mistake is to perform a heavy computation inside a promise (or async function) thinking it won’t block – but remember, all JavaScript execution in a single Goja runtime is on one thread. If you do something CPU-intensive (say, calculate Fibonacci of a huge number) in an async function *without* breaking it up, you will block the event loop just the same as if it were synchronous. The `async` keyword doesn’t make the function run in a separate thread; it just makes it easier to yield the thread at await points. So, for CPU-bound tasks, consider offloading them to a Go goroutine and then resolving a promise, rather than looping in JS for a long time.

**Mixing Go Concurrency with JS Promises:** If you spawn many goroutines that each schedule promise resolutions or callbacks via `RunOnLoop`, those will all queue up on the single JS thread. This is fine – it’s effectively a producer (goroutines) -> consumer (JS loop) model. But be mindful of back-pressure: if goroutines produce events faster than the JS thread can consume, the queue could grow and memory usage could increase. In extreme cases, you might need semaphore or rate-limit mechanisms on the Go side. This is similar to how one must be cautious with Node.js if many events are emitted rapidly; the single-threaded listener must handle them promptly.

In summary, Goja’s support for promises and async/await allows you to write idiomatic modern JavaScript code (with `.then`, `catch`, and `await`) in your embedded scripts. The main limitations are the need for an event loop to drive these asynchronous operations and the lack of certain newer JavaScript features (like top-level await and async generators). By understanding these constraints and using the event loop properly, you can effectively use promises in Goja. Next, we’ll turn to how to integrate Go’s own concurrency (goroutines) with this model in a safe and structured way.

## Chapter 5: Integrating Go Goroutines with Goja’s Single-Threaded Model

A major challenge when embedding Goja is **bridging Go’s concurrency (goroutines) with the JavaScript event loop**. As we've emphasized, a Goja runtime cannot be accessed from multiple goroutines at the same time. Yet, we want to utilize Go’s ability to do work in parallel (database calls, computations, network requests) and then feed results back into the JavaScript environment. The solution is to combine what we learned in previous chapters: use Go goroutines for parallel tasks, and use the event loop (or other synchronization) to safely send results back to the JS thread.

### 5.1 The Golden Rule: One Thread for JS, Communicate via Queue

Think of the Goja runtime as a single-threaded event loop (like the main thread in a browser). We spawn goroutines to do background work, but they **must not** directly manipulate the JS state. Instead, any interaction – calling a JS function, resolving a promise, modifying a global variable – should be done by posting a task to the JS event loop.

**Use Channels or Loop for Communication:** In simpler integration scenarios, you might not use the full `EventLoop` package but still need to synchronize access. For example, you could have a single goroutine running `vm.RunString` in a loop picking up tasks from a Go channel. That channel acts as a task queue. This is effectively a DIY event loop. The `goja_nodejs/eventloop` is just a convenient, well-tested implementation of such a queue with extras (timers, etc.). For most purposes, it’s recommended to use `EventLoop` rather than building your own, but the concept is the same.

### 5.2 Pattern: Running Multiple Goja Instances for Parallelism

One straightforward way to leverage multiple CPU cores or to handle truly simultaneous JS executions is to run **multiple Goja runtimes in parallel**. Since each Goja `Runtime` is independent, you could for example spin up a pool of N runtimes (each with its own event loop) and distribute work among them. Each one will still be single-threaded internally, but you get parallel execution across different runtimes on different goroutines.

A real-world example of this approach is from the PocketBase project, which uses Goja for scripting. Initially, they tried to call a single Goja runtime from multiple goroutines and encountered the thread-safety issues. They then refactored their design such that “each route, middleware, etc. handler, that is usually running in its own goroutine, is executed as a standalone Goja program”. In effect, each request handled by PocketBase got its own isolated Goja runtime execution, avoiding any concurrent access to the same VM. This is a viable design pattern: **isolate concurrent tasks by giving each its own VM**, if the tasks don’t need to share a lot of state.

The downside of multiple runtimes is higher memory usage and the complexity of sharing state between them if needed (which often requires serialization or external storage). If your use-case involves many small isolated scripts (like user-provided scripts that run per event), multiple runtimes could be fine. If instead you have a single script/application that needs to handle many async events (like a Node.js app would), a single runtime with an event loop is more appropriate.

### 5.3 Safe Data Exchange and State Management

If you need to communicate data between Go and JS, or between multiple JS runtimes, careful planning is needed for thread safety:

* **Primitive data and copies:** It’s safe to pass copies of data (like strings, numbers, or even JSON-serialized objects) between goroutines. For example, a goroutine could prepare a JSON string result, send it through a channel to the event loop thread, which then parses it in JS.
* **Go objects exposed to JS:** You can store a pointer to a Go struct in a Goja runtime (via `vm.Set("obj", somePointer)`). If that object is to be used by JS, and *also* possibly accessed by other goroutines, you must guard its access with mutexes or other sync primitives on the Go side. Another approach is to design those Go objects to be mostly immutable or thread-safe (e.g., a concurrent map or a read-only configuration struct). Sharing such an object is okay as long as its methods are safe to call concurrently. But remember, when JS calls a method on a Go object, that call happens on the JS thread (synchronously as part of JS execution), so it won’t overlap with other JS operations – the concurrency concern is only if some other goroutine is calling the same object’s methods at the same time.
* **Avoid sharing Goja Values across threads:** Never take a `goja.Value` or `*goja.Object` from one runtime (or one thread) and try to use it on another. Goja values are tied to their `Runtime`. Instead, if you need to transfer something, convert it (e.g., export to a Go value, then send, then create a new JS value in the other runtime). The documentation explicitly notes that you cannot pass object values between runtimes.

### 5.4 Using Mutexes vs. Event Loop for JS Access

A question arises: could we just put a mutex around all calls into a single Goja runtime, allowing goroutines to take turns calling it? While theoretically possible (some have attempted wrapping `vm.RunString` or `vm.Call` with a lock), this is error-prone and can lead to deadlocks or missed context. It’s safer to funnel through an event loop or channel because it preserves ordering and context.

For example, if goroutine A and B both try to call JS, with a naive mutex approach A might lock, start a long JS execution, then B blocks. If during A’s JS execution, a promise resolution from some earlier async needs to be processed, that could complicate matters if not done carefully. The event loop model naturally handles this by queueing B’s request behind A’s already running task, and also injecting promise jobs in the right places.

In summary, the event loop is effectively a specialized mutex+queue tailored for JS tasks. Use it instead of manual locks for interacting with the JS runtime, whenever possible.

### 5.5 Example: Combining Goroutines and Event Loop

Let’s revisit an earlier scenario with a slightly more complex example: Suppose you want to allow JavaScript to request data from a Go HTTP client asynchronously. You decide to add a JS function `httpGet(url)` that returns a Promise which resolves with the response body.

Implementing this:

```go
vm := loop.Runtime()
vm.Set("httpGet", func(call goja.FunctionCall) goja.Value {
    url := call.Argument(0).String()
    promise, resolve, reject := vm.NewPromise()
    // Launch Go routine to perform HTTP GET
    go func() {
        resp, err := http.Get(url)
        if err != nil {
            // Call reject on JS thread
            loop.RunOnLoop(func(vm *goja.Runtime) {
                reject(vm.ToValue(err.Error()))
            })
            return
        }
        body, err := ioutil.ReadAll(resp.Body)
        resp.Body.Close()
        if err != nil {
            loop.RunOnLoop(func(vm *goja.Runtime) {
                reject(vm.ToValue(err.Error()))
            })
            return
        }
        // Successfully got body, resolve the promise with the body string
        loop.RunOnLoop(func(vm *goja.Runtime) {
            resolve(vm.ToValue(string(body)))
        })
    }()
    return promise
})
```

Now a JavaScript script can do:

```javascript
async function fetchExample() {
    try {
        let data = await httpGet("https://example.com/data.json");
        console.log("Data received:", data);
    } catch (err) {
        console.log("Request failed:", err);
    }
}
fetchExample();
```

This pattern is robust and safe:

* Multiple concurrent `httpGet` calls can be made from JS (the JS code can call `httpGet` several times without waiting since it gets back promises).
* Each call spawns a separate Go goroutine doing the HTTP request in parallel.
* Each goroutine, when finished, enqueues either a resolve or reject on the JS event loop. These will execute one by one on the single JS thread, resolving each Promise and triggering the corresponding `.then()` or the awaiting function’s continuation.
* Because of the event loop, even if responses come back near-simultaneously, their handlers run in a controlled sequential order on the JS side (the order of completion will be preserved, or at least each callback is atomic on the JS thread).

**Pitfall – Forgetting to `RunOnLoop`:** If in the above example we mistakenly called `resolve` directly inside the goroutine (not using `RunOnLoop`), the program would likely panic with a runtime error or produce incorrect behavior. This is a classic mistake. Always test that your integration functions work by trying concurrent calls. If there’s any data race or misuse, Go’s race detector or explicit panics (like index out of range in Goja internals) will surface.

**Graceful Shutdown Consideration:** When your Go program needs to shut down, and you have an event loop running, make sure to stop the loop (via `Terminate()` or `Stop()`). The event loop might be running in the background preventing the program from exiting if not stopped. Also, if goroutines may still try to post to a terminated loop, you might want to coordinate their lifetimes (for instance, check `loop.RunOnLoop`’s return value – it returns false if the loop is terminated, so you could handle that scenario in your goroutine by not calling resolve/reject because the runtime is gone).

In this chapter, we saw that the key to integrating goroutines with Goja is **strict separation of concerns**: goroutines do the concurrent work, and the Goja event loop (single thread) handles all interactions with the JS state. By following this rule, you can avoid data races and crashes, and make full use of Go’s concurrency to perform asynchronous tasks that ultimately feed results into the JavaScript environment.

## Chapter 6: Using the `eventloop` Package – Timers, Tasks, and More (In-Depth)

We introduced the `goja_nodejs/eventloop` package earlier, but in this chapter we will dive deeper into its features and how to use it effectively in more complex scenarios. The event loop is the backbone for managing asynchronous operations in a Goja runtime; understanding its functions in depth will allow you to implement patterns similar to Node.js within Goja.

### 6.1 Starting and Stopping the Event Loop

When using an `EventLoop`, a common pattern is:

```go
loop := eventloop.NewEventLoop() 
loop.Start()
// ... schedule tasks ...
// eventually:
loop.Stop() 
```

A few notes on these:

* **`NewEventLoop(opts...)`:** You can pass options. One useful option is `eventloop.EnableConsole(true/false)` which controls whether the `console` module is loaded in the JS runtime by default. By default it’s true, meaning you get `console.log` and friends. You can disable it if you want a quieter or custom console.
* Another option is `eventloop.WithRegistry(reg)` where you can provide a `require.Registry` for module loading (more on modules in the next chapter). This allows sharing a module cache or preloading native modules.
* **`StartInForeground()`:** Use this if you want to run the event loop on the current goroutine and be able to catch panics from tasks. For example, in a testing or REPL scenario, you might do `loop.StartInForeground()` so that if a JS task calls a Go function that panics, you can recover it outside of `StartInForeground`. If you run with `Start()` (background), a panic in a JS callback (originating from Go code) will by default crash the program unless that Go code itself recovers. `StartInForeground` gives you a chance to handle it.
* **`Stop()` vs `Terminate()`:** `Stop()` will wait for the loop to become idle (no pending tasks except possibly scheduled timers) and then stop. `Terminate()` will immediately halt the loop and clear any scheduled timers. `StopNoWait()` is a variant that signals the loop to stop after the current tasks without waiting (and can be called from within the loop). Use `Terminate()` if you want to forcefully shutdown (for instance, on program exit) and not even execute remaining timers. Use `Stop()` if you want to gracefully let it finish work.

**Example:** If you had an interval running and you call `Stop()`, the loop will not actually stop until that interval callback finishes and no other tasks remain. If the interval keeps scheduling itself (which it will by nature), `Stop()` might never stop (thus you would call `ClearInterval` first, then `Stop()`). In contrast, `Terminate()` would break out immediately, even if an interval is pending (that interval simply won’t run).

### 6.2 Coordinating Multiple Events and Timers

You can schedule multiple different timers and immediate tasks. They will execute in order of their scheduling times:

* Any tasks scheduled via `RunOnLoop` without delay will run as soon as the current running task (if any) yields.
* Timers (`SetTimeout`) will run after their specified duration, possibly interleaved with other tasks if those tasks are scheduled to occur earlier.
* If two timers are set for the same time, the one set first will run first (the event loop preserves insertion order for tasks scheduled at the same time or tick).

**Example Scenario:** Suppose you schedule:

```go
loop.SetTimeout(func(vm *goja.Runtime) {
    fmt.Println("A")
}, 100*time.Millisecond)
loop.SetTimeout(func(vm *goja.Runtime) {
    fmt.Println("B")
}, 50*time.Millisecond)
loop.SetTimeout(func(vm *goja.Runtime) {
    fmt.Println("C")
}, 50*time.Millisecond)
```

We expect "B" and "C" to both fire at \~50ms, and "A" at \~100ms. The ordering between B and C: since B was scheduled before C at the same timeout, B’s callback will execute before C’s. So output would be:

```
B
C
A
```

This ordering guarantee (preserving the order of scheduling for same-time tasks) is mentioned in the `RunOnLoop` documentation and generally holds for timers as well.

**Idle Time:** If the event loop has no tasks, it will wait (if `Start()` in background, the goroutine is blocked waiting; if `StartInForeground()`, it would block the current goroutine). When a new task or timer is added, it wakes up and processes it. This is all managed internally, so from the user perspective, you typically don’t have to worry about it. Just know that a running loop with nothing to do will not consume CPU significantly – it’s waiting on a condition or timer internally.

### 6.3 Example: Implementing an Async Task Queue in JS

To illustrate the use of the event loop in a more complex way, consider implementing a simple task queue in JavaScript, where JS code can push tasks to be done “later” without specifying a time (like `setImmediate` in Node, which runs a task on the next tick).

We can simulate `setImmediate` in Goja using `RunOnLoop`:

1. Expose a global JS function `setImmediate(func)` that takes a callback.
2. When called, that JS callback should be scheduled via `loop.RunOnLoop`.

In Go, this might look like:

```go
vm := loop.Runtime()
vm.Set("setImmediate", func(call goja.FunctionCall) goja.Value {
    cbVal := call.Argument(0)
    // Ensure it’s a function:
    fn, ok := goja.AssertFunction(cbVal)
    if !ok {
        panic(vm.ToValue("setImmediate: callback is not a function"))
    }
    // schedule the function to run on the loop
    loop.RunOnLoop(func(vm *goja.Runtime) {
        fn(goja.Undefined()) // call with no 'this' and no arguments
    })
    return goja.Undefined()
})
```

Now from JavaScript, if code calls:

```javascript
console.log("Task 1");
setImmediate(() => console.log("Task 3"));
console.log("Task 2");
```

The expected output is:

```
Task 1
Task 2
Task 3
```

Because `setImmediate` schedules the callback to run after the current synchronous code, effectively queuing it for the event loop. The above implementation does exactly that: it enqueues the callback to be executed on the loop as soon as possible after the current turn.

This example shows how the event loop lets us create higher-level concurrency primitives in JS.

**Simulating I/O or Long Operations:** We already saw how to integrate actual I/O (HTTP, file read) by using goroutines and `RunOnLoop`. If you want to simulate a non-blocking operation entirely in JS (for testing or other reasons), you could use `SetTimeout` with 0 delay as a trick (similar to `setTimeout(fn, 0)` in browsers). That effectively defers execution of `fn` until the current call stack is cleared. The eventloop doesn’t have a dedicated `setImmediate` function (as Node has), but using a combination of `RunOnLoop` and `SetTimeout` can achieve similar results.

### 6.4 Debugging the Event Loop

**Logging and Console:** The event loop by default provides a `console` object (if not disabled). `console.log` prints to stdout. If you need more sophisticated logging (like timestamps or levels), you may implement a custom console by disabling the built-in one and providing your own global `console` object from Go. For example, you could override `console.log` to write to your application’s logger instead.

**Inspecting Tasks:** There’s no official API to list pending tasks or timers in the event loop (just like in Node, you don’t normally peek into the event loop’s internals at runtime). If debugging, you might instrument your code by printing when scheduling tasks. For instance, wrap `RunOnLoop` calls in a helper that logs what is being scheduled. Similarly, you can log when a timer callback runs, by simply adding a `fmt.Println` in the Go function or using `console.log` in the JS callback itself.

**Common Mistake – Forgetting to Stop:** If you start an event loop in a program that otherwise would exit (for example, in a `main` function after doing some work), forgetting to call `Stop()` or `Terminate()` will cause the program to hang. The event loop’s background goroutine will keep running (idle, but waiting). Always ensure you stop the loop if your program is ending or if you want to release resources. One way is to use `defer loop.Stop()` right after starting it, as shown in earlier examples. That ensures cleanup even if a panic or return happens.

**Error Handling in Scheduled Functions:** If a panic occurs inside a function that was run via `RunOnLoop` or a timer, by default it will bubble up and **stop the event loop goroutine** (since it’s not recovered internally in the loop). This is often not what you want in a long-running process. To handle this:

* Use `StartInForeground` and recover outside of it. But that only works if you run the loop in the main goroutine.
* Alternatively, wrap your callback logic in a `recover` block. For example:

  ```go
  loop.SetTimeout(func(vm *goja.Runtime) {
      defer func() {
          if r := recover(); r != nil {
              fmt.Println("Recovered panic in timeout:", r)
          }
      }()
      // ... callback logic ...
  }, 1*time.Second)
  ```

  This way, a panic in that specific callback won’t kill the whole loop.
* Another advanced approach is to use Goja’s ability to catch exceptions if the panic came from a JS throw or a Go panic with a `goja.Value`. If the panic is a plain Go panic (like a nil pointer dereference), you have to recover in Go as above.

The event loop is a powerful tool to simulate an entire asynchronous runtime for Goja, essentially turning Goja into a Node.js-like environment where JavaScript code can use familiar patterns (timers, callbacks, promises) to manage concurrency. Mastery of the event loop is key to building complex asynchronous JavaScript behaviors on top of Go’s concurrency.

Now that we’ve covered the event loop in detail, including how to integrate it with Go concurrency and how to use it for common async patterns, let's move on to integrating native modules and providing additional functionality to the JavaScript code via the `require` system.

## Chapter 7: Integrating Native Go Modules and Exposing Asynchronous APIs

One of Goja’s strengths is the ability to *extend* the JavaScript runtime with modules and functions implemented in Go. This allows you to expose Go libraries and system calls to your JS code, much like Node.js exposes native (C/C++) modules or its own standard library to JavaScript. In this chapter, we’ll discuss how to use the `require` system in Goja (provided by `goja_nodejs/require`) to load modules, how to register your own native modules, and how to design these modules – especially those that need to perform asynchronous work.

### 7.1 The `require` System in Goja

Goja’s Node.js compatibility library includes a CommonJS-like module loader. By enabling it, you allow JavaScript code to use `require("moduleName")` to load either:

* Built-in core modules (like `'fs'` or `'util'` if they are implemented in `goja_nodejs`).
* Local JavaScript files (just like requiring a `.js` file in Node).
* Native modules (written in Go, registered via the API).

**Enabling require:** From the earlier snippet in the goja\_nodejs README, using the require system typically looks like:

```go
import "github.com/dop251/goja_nodejs/require"

registry := require.NewRegistry()             // create a module registry
req := registry.Enable(runtime)               // enable require() in the given runtime
resultVal, err := runtime.RunString(`var x = require("./someModule.js");`)  // example usage
```

When you call `registry.Enable(runtime)`, it sets up the global `require` function in that runtime’s JS environment. The `Registry` keeps track of modules that have been loaded, caches them, and knows how to find modules on the filesystem. By default, `require` will look in the current working directory and node\_modules folders for modules, similar to Node’s algorithm (the implementation follows Node’s module resolution rules for the most part).

**Core Modules:** The `goja_nodejs` library comes with a few core modules (as seen in its repository file list: e.g., "buffer", "console", "process", "util", etc.). These are automatically registered as core modules. For example, `require('util')` or `require('buffer')` will return a JS object implementing those Node APIs. If you only need basic JS and some console logging, you might not even need many modules. But if you want file system access, note that `fs` is not implemented by default in goja\_nodejs at the time of writing – you would have to implement it as a native module or manually via Go functions.

**Loading Local JS Files:** Suppose you have a JavaScript file `math.js` with content:

```javascript
// math.js
exports.add = function(a, b) { return a + b; }
```

If your Go program’s working directory has `math.js`, then in Goja:

```javascript
const math = require("./math.js");
console.log(math.add(2,3)); // should print 5
```

This would work. The require system compiles `math.js` and caches the exports. Note: the file path resolution needs `./` for a relative file or just `math` would search node\_modules.

### 7.2 Registering Native Go Modules

**Native Module** means a module that is implemented in Go code instead of JS code. For example, you might want a module called `"os"` to expose some operating system functions like reading environment variables.

To register a native module, use `require.RegisterNativeModule` **before** enabling the registry or at least before calling `require()` for it. This function can be called at package init or in main before running scripts. There are two forms:

* A global registration via `require.RegisterNativeModule("moduleName", loaderFunc)`. This makes the module available in any registry.
* A registry-specific one via `registry.RegisterNativeModule("moduleName", loaderFunc)`. If you want to isolate modules per registry.

The **ModuleLoader** function signature is:

```go
func(*goja.Runtime, *goja.Object)
```

It is called when the module is required, with the target runtime and a `module` object. In Node, modules are typically implemented by populating `module.exports`. Here, the `module` object passed in has a property `exports` that is initially an empty object. The loader function should populate `module.Get("exports")` or replace it with whatever the module should export.

**Example – Creating a Native Module:**

Let’s create a native module `"env"` that provides access to environment variables:
We want `require('env')` in JS to return an object with a function `get(name)` that returns `process.env[name]`, and maybe `set(name, value)`.

In Go:

```go
require.RegisterNativeModule("env", func(runtime *goja.Runtime, module *goja.Object) {
    // Get the exports object
    exports := module.Get("exports").(*goja.Object)

    // Define a Go function for getting env var
    exports.Set("get", func(call goja.FunctionCall) goja.Value {
        key := call.Argument(0).String()
        val := os.Getenv(key)
        return runtime.ToValue(val)
    })
    // Define a Go function for setting env var
    exports.Set("set", func(call goja.FunctionCall) goja.Value {
        key := call.Argument(0).String()
        value := call.Argument(1).String()
        err := os.Setenv(key, value)
        if err != nil {
            // throw JS exception
            panic(runtime.ToValue(err.Error()))
        }
        return goja.Undefined()
    })
})
```

This should be done before any script calls `require('env')`. Typically, you would do it in an `init()` function or at the start of `main`:

```go
func init() {
    require.RegisterNativeModule("env", envModuleLoader)
}
```

Now, from JavaScript:

```javascript
const env = require("env");
env.set("DEBUG", "true");
console.log(env.get("DEBUG")); // "true"
```

This module interacts with OS environment variables through Go.

Notice how we handled errors: if `os.Setenv` returns an error, we used `panic(runtime.ToValue(err.Error()))`. In Goja, if a Go function panics with a value that is a `goja.Value`, the engine will convert that into a JavaScript exception that can be caught in JS. So our `env.set` will throw a JS Error string if setting the env fails, which the JS code could catch.

**Returning Complex Data:** If your module needs to return something other than a simple object, you can set `module.Set("exports", someValue)` to completely override exports. For instance, if you wanted `require('env')` to yield directly a function, you could do:

```go
module.Set("exports", runtime.ToValue(yourFunction))
```

Then `const env = require('env');` would make `env` a function.

**Asynchronous APIs in Modules:** If you want to expose an async API via a module, you would use the patterns from earlier chapters. For example, say we want a module `'net'` with a function `fetch(url)` that returns a Promise for the HTTP GET. We can implement that similarly to the `httpGet` function we wrote before, but encapsulated in a module:

```go
require.RegisterNativeModule("net", func(runtime *goja.Runtime, module *goja.Object) {
    exports := module.Get("exports").(*goja.Object)
    exports.Set("fetch", func(call goja.FunctionCall) goja.Value {
        url := call.Argument(0).String()
        // Use runtime.NewPromise to create promise inside this runtime
        promise, resolve, reject := runtime.NewPromise()
        go func() {
            resp, err := http.Get(url)
            if err != nil {
                loop.RunOnLoop(func(vm *goja.Runtime) {
                    reject(vm.ToValue(err.Error()))
                })
                return
            }
            body, err := ioutil.ReadAll(resp.Body)
            resp.Body.Close()
            if err != nil {
                loop.RunOnLoop(func(vm *goja.Runtime) {
                    reject(vm.ToValue(err.Error()))
                })
            } else {
                loop.RunOnLoop(func(vm *goja.Runtime) {
                    resolve(vm.ToValue(string(body)))
                })
            }
        }()
        return promise
    })
})
```

The above assumes you have access to the `loop` variable (the EventLoop) in that scope. You might need to close over it or have it global. This design choice is up to your application structure. The key is that the module uses `runtime.NewPromise()` to create a Promise for that specific runtime (since modules can be loaded in multiple runtimes if you had multiple VMs, the `runtime` parameter ensures we create a promise in the correct one).

One more subtlety: inside the goroutine, we used `loop.RunOnLoop`. That `loop` must correspond to the event loop running for *that* runtime. If you only have one event loop and one runtime, this is straightforward. If you had separate event loops, you might store in a map from runtime to its loop, or use `WithRegistry` to attach a registry to a specific loop’s runtime.

**Testing the async module:**

```javascript
const net = require("net");
net.fetch("https://example.com").then(body => {
    console.log("Fetched bytes:", body.length);
}).catch(err => {
    console.error("Fetch failed:", err);
});
```

This should print the length of the fetched content or an error. The `.then` works because our `fetch` returned a real JS Promise object and we resolved it correctly on the loop thread.

### 7.3 Managing Module State and Concurrency

One thing to be cautious about is that the module registry caches modules. If you call `require("env")` multiple times, it will return the same exported object each time (just like Node caches modules after first load). This means if your module has internal state (like counters or open connections stored in its exported object), that state will be shared across all requires in the same runtime/registry.

If you have multiple event loops (each with its own `Registry` or if you share one registry among them), consider whether the module should be singletons across runtimes or not:

* If you use a single `Registry` shared by multiple runtimes (via `WithRegistry` when creating event loops), then a native module registered globally will be shared among them. For example, our `env` module uses the OS environment which is global anyway, so sharing is fine. But if we had a module that, say, manages a connection to a database, sharing one instance across two runtimes might cause concurrency issues if both runtimes use it at the same time.
* The PocketBase documentation we saw warns that loaded modules use a shared registry and **mutations should be avoided** to prevent concurrency issues. This is important: if two separate JS handlers (on different goroutines with separate runtimes) require the same module from a shared registry, they will get the same module object (the same Goja *Object* pointer). If those two runtimes are running concurrently on separate threads, that is a big no-no – a Goja `Object` cannot be used from two runtimes or threads at once. To avoid this, you have a few strategies:

  * Do not share the registry; instead, create a new Registry per runtime. This means each runtime will load its own instance of the module, avoiding cross-thread usage. It uses more memory but isolates state.
  * If sharing a registry for memory reasons, ensure that the modules are effectively read-only or immutable after initialization. For example, a module that only provides pure functions or constants is okay to share, because calling those functions in separate runtimes still executes in each runtime’s context (the functions might be Go functions which are safe for concurrent use if they avoid altering shared data).
  * If a module does hold mutable state and you want it global (like a singleton service), you must guard it with locks internally. Perhaps that module uses a sync.Mutex around its operations. But also you have to be careful that the *JS object representing the module* isn't being manipulated concurrently. Usually it won’t be, because each runtime will operate on its own `exports` object; however, if the registry is shared, they might actually be sharing the exact same object reference. That scenario is best avoided.

In summary, for concurrency safety:

* **Prefer separate registries per runtime** unless you have a clear reason to share (like large libraries you don’t want to compile multiple times).
* If you share, design modules as stateless or thread-safe. Avoid modules that accumulate state that multiple scripts might touch concurrently.

### 7.4 Best Practices for Module Design

* **Keep modules focused:** Just like in Node, a module should ideally encapsulate one area (e.g., a database module, a crypto module, etc.). This makes it easier to manage their state and testing.
* **Use Promises for async operations:** If your module function does I/O or other delayed work, consider making it return a Promise and using the patterns we’ve covered. This way JavaScript users can use `await` or `.then` to handle results. It’s more idiomatic than old-style callbacks in modern JS.
* **Document the usage:** If others (or future you) use the modules, make clear which ones are synchronous vs asynchronous, and how errors are signaled (exceptions, promise rejections, etc.).
* **Register modules in `init`:** By registering native modules in the `init()` function of your Go package (that uses goja), you ensure they’re ready by the time you create a runtime and enable require. This also keeps the registration code organized by module.
* **Clean up if needed:** If a module holds external resources (file handles, network connections), you might need a way to clean up on program exit or loop termination. Goja doesn’t have an automatic module unload (once required, modules live until the runtime is garbage-collected), so you may have to close connections either on a certain event or rely on Go’s process exit to clean up (which isn’t ideal for long-running processes). Another pattern is to not hold persistent connections in the module; instead, expose functions to connect/disconnect and let JS code manage when to call them.

By integrating native Go modules, you can expose a wide array of functionalities to the JavaScript code running in Goja. This is how you “extend” the JS environment to make it more than just a calculator – you can give it the ability to read files, make HTTP requests, access databases, etc., all through carefully written Go code. In doing so, always keep concurrency and safety in mind, as the boundary between Go and JS is where a lot of subtle bugs can arise if not designed properly.

Now that we have concurrency, event loops, and modules all in our toolkit, let’s move to some advanced topics: performance tuning, ensuring state safety, and dealing with errors that cross between Go and JavaScript.

## Chapter 8: Advanced Topics – Performance, State Safety, and Error Handling

In this chapter, we address several advanced aspects of using Goja in concurrent scenarios: how to tune and scale for performance, strategies for maintaining state consistency and avoiding race conditions, and handling errors that occur in asynchronous code across the Go-JS boundary.

### 8.1 Performance Tuning and Scaling Concurrency

**Single-threaded Bottleneck:** Remember that one Goja runtime = one OS thread (conceptually) executing JavaScript. No matter how many goroutines you spawn to feed tasks, the tasks still queue up on that single thread. This means the **throughput of JS execution is limited by the speed of one core**. If your application is CPU-bound on the JavaScript side, you won’t get more performance by adding goroutines – you would need to either optimize the JS code or run multiple JS runtimes in parallel (which introduces complexity in sharing data).

**Multiple Runtimes for Parallelism:** As discussed, you can partition work across multiple runtimes to utilize multiple cores. For example, if you have a heavy computation that can be parallelized, you could spin up N Goja runtimes and dispatch parts of the work to each, then combine results. This is akin to the cluster module in Node (multiple processes). The downside is each runtime has its own heap and context, so splitting work requires serializing data (e.g., to JSON or basic types) to send to another runtime. For compute-heavy tasks, this overhead might be acceptable.

**Use Go for Heavy Lifting:** If a complex calculation can be done in Go more efficiently, consider doing it in Go and only giving the result to JS. Go’s performance (with native types or using optimized libraries) will often beat an interpreted JS doing the same algorithm. For example, if you need to sort a large array of numbers, it might be faster to export it to Go, sort there (concurrently if possible), then bring it back. This hybrid approach leverages Go’s strength in concurrency and heavy computation, while letting JS handle high-level logic or scripting.

**Avoid Unnecessary Conversions:** Every time you move data between Go and JS, there’s overhead. Goja has to convert Go values to JS `Value` and vice versa. If you can, try to keep more logic on one side to reduce back-and-forth. For instance, if you have to process a large list, doing the entire loop in JS might be slow; doing it in Go might be faster, but that means copying the list to Go (Export) and then back. Instead, maybe implement a custom Go function to iterate and call a JS callback per item – but be careful, that would call back into JS repeatedly and could thrash performance due to frequent cross-boundary calls. This is a design decision: measure what’s better for your use-case.

**Memory and GC Considerations:** Goja’s garbage collector is separate from Go’s. Large allocations in the JS heap will put pressure on Goja’s GC, which runs stop-the-world for the JS VM (but doesn’t stop the whole program, only the JS execution). If you find GC pauses in JS problematic, see if you can stream or chunk data rather than handling huge arrays in JS memory at once. Also note that Go’s GC doesn’t see inside the JS heap, and Goja’s GC doesn’t know about Go. However, if you create a lot of transient Goja objects, they will be garbage-collected by Goja – that’s fine. If you create a lot of Go objects or allocate through cgo or such, that’s separate.

**JIT vs Interpreter:** Goja is an interpreter (no JIT), so it’s not as fast as V8 or QuickJS JIT for raw computation. But it’s usually fine for many tasks. If you do need extreme performance for JS code, consider whether you really need to do that part in JS. Some teams use WebAssembly or other techniques to speed up heavy logic in a JS environment.

### 8.2 State Safety and Data Race Avoidance

**No Shared Mutable State between JS and Go without Locks:** If you have a Go data structure that is accessible both from JS (via an exported object) and from other goroutines, you absolutely must guard it. For example, a `sync.Map` can be safely accessed by multiple goroutines, including one from a JS call and others from Go. But a plain map or slice you expose to JS, if you also modify it from Go concurrently, is not safe. Either convert it to something like `sync.Map` or funnel all modifications through the JS thread via `RunOnLoop`.

**Atomicity of JS operations:** JavaScript operations on its own data structures are atomic from the perspective of JS’s single thread (there’s no concept of another thread modifying a JS object at the same time). But if you, from Go, poke a JS object’s property via `runtime.Set` or `Object.Set` while the JS is running, that could be a race if done concurrently. Always schedule such pokes on the event loop.

**Immutability and Copies:** An approach to avoid shared state issues is to work with **copies and immutable data**. For example, if JS passes an object to Go, instead of Go holding a pointer to that object and possibly accessing it later, maybe Go immediately copies out the necessary fields (via `Export()` to Go struct). Then Go’s background goroutine works on that copy, which JS can’t change under its feet. Similarly, when sending data to JS, sometimes it’s safer to send a copy. For instance, if a background goroutine produces a slice of results, rather than populating a JS Array in pieces, it can build a Go slice and then, once complete, do `vm.ToValue(slice)` in one scheduled step. That way, JS sees a final array and there’s no moment where the array is half-filled by a concurrent process.

**AsyncContextTracker and Context Propagation:** For advanced use, Goja’s `AsyncContextTracker` allows tracking context across async calls. This is useful for things like logging correlation, or tracking a request ID through asynchronous callbacks. Implementing `AsyncContextTracker` means writing a struct that implements the interface with `Grab()`, `Resumed()`, `Exited()` methods. For example, you could store the current request ID in a context and have `Grab()` return it whenever a promise is about to be awaited, then `Resumed` would set that ID into some global or context for when the promise callback runs. This is akin to Go’s context or Node’s AsyncLocalStorage. If you have scenarios where a lot of asynchronous callbacks need to know some ambient state (like the user ID under which the code is running), AsyncContextTracker can help maintain that consistently across awaits. Without it, you might lose track of context when code goes async.

**Global Data in Modules:** If you have modules or global objects that multiple parts of code use, consider whether they need locking. For instance, a global cache object (a JS Map or a Go map exposed to JS) could become a bottleneck or a race if misused. One trick if you need a globally accessible thing and want to avoid locks is to also confine its use to the event loop. E.g., to update a global config, always do it through a JS function (which will run on the loop). That way, even if multiple sources update it, they serialize on the JS thread.

### 8.3 Error Propagation Across Async Boundaries

Error handling in an asynchronous environment can be tricky. Let’s break down scenarios:

* **JavaScript exception to Go:** If a JS function throws (or a promise rejects without catch), and that propagates out to Go (for example, you call `vm.RunString` and it throws), Goja returns an `error` of type `*goja.Exception`. You can inspect that to get the thrown value. We saw earlier how you can extract the message or value. The stack trace can also be obtained (the Exception might have a Stack or you can call `exception.Value().String()` which often includes the message and stack). When writing Go code that calls into JS, always consider the possibility of an exception and handle the error. If you don’t, it may bubble up and possibly terminate your program if not caught. For long-running servers, you probably want to catch exceptions, log them, maybe send them to the client or convert them to a Go error.

* **Go error to JavaScript:** If a Go function called from JS returns an error (like a `func() (int, error)` exported via ExportTo) or panics with a `goja.Value`, those become JS exceptions. Example: our `env.set` function panicked with a goja Value containing an error string, which means in JS you could do:

  ```javascript
  try { env.set("X", "Y"); }
  catch(e) { console.log("Failed:", e); }
  ```

  and it would catch the “Failed: some error” message. If a Go function instead returns an `error` as a second result, Goja will automatically turn a non-nil error into a thrown exception in JS (this was indicated by the panic we saw when someone didn’t handle a call properly – it resulted in runtime panic, but generally, ExportTo with function signatures will map errors to exceptions). So design your Go-to-JS functions either to handle errors internally or to clearly document that they throw.

* **Promise Rejections:** If an error happens inside an async function (e.g., our `fetch` sets `reject(err)`), that error will surface as a rejected promise in JS. It won’t be thrown in JS unless you `await` it without try/catch or call `.then` without `.catch`. Encourage users of your API to use proper promise error handling. Or use `SetPromiseRejectionTracker` in development to log warnings for unhandled rejections.

* **Cancellation and Timeout:** Sometimes you might want to cancel an ongoing async operation (say a network request taking too long). You can model this with promise cancellation patterns. Goja doesn’t have built-in promise cancellation, but you can incorporate it by, for example, having your Go function check a context. If you plan for cancellation, one approach is to attach an abort signal object in JS. For example, a module could export `fetch(url, { signal })` where `signal` is an object with an event or method to trigger cancellation. The Go code could watch for that (maybe use `signal` to call back into Go or use some atomic flag). Alternatively, if using context.Context in Go, you could store context in a map and expose a `cancel` JS function that calls that context’s CancelFunc. This is an advanced design and beyond Goja’s provided API, but doable with careful coordination.

* **Handling Panics in Event Loop:** We touched on it: a panic in a `RunOnLoop` or timer callback will crash the loop if not caught. The best practice is to catch errors in any top-level scheduled function:

  ```go
  loop.RunOnLoop(func(vm *goja.Runtime) {
      defer func() {
          if r := recover(); r != nil {
              // Convert or log the panic
              var jsErr goja.Value
              if val, ok := r.(goja.Value); ok {
                  jsErr = val  // it was a JS exception or a Value panic
              } else {
                  jsErr = vm.ToValue(fmt.Sprint(r))
              }
              fmt.Println("Error in async callback:", jsErr)
          }
      }()
      // ... do work that might panic or throw
  })
  ```

  If the panic was a `goja.Exception` (like a JS throw not caught in JS), it will be recovered as a `goja.Value` which you can inspect as above. This way your event loop can continue running after an error in a task, which is important for robustness (much like an uncaught exception in Node will typically crash the process – but in our case, we have the power to decide this).

* **Cross-boundary Stack Traces:** If an error originates in JS, its stack trace is within JS. If it originates in Go (like a panic or error), you might only get a Go stack in logs which isn't directly meaningful to JS developers. One idea is to annotate errors. For instance, if a Go function might produce an error due to a JS-supplied invalid argument, you could do `reject(vm.ToValue(fmt.Sprintf("GoError: %v", err)))` to give a hint. Also, consider using `runtime.CaptureCallStack` if needed to get JS stack at a point and attach it to errors.

In conclusion, handling errors gracefully in a mixed Go-JS environment means catching exceptions at boundaries, using promise rejection tracking, and possibly instrumenting tasks with recovery. It’s better to catch and log than to let the whole program crash (unless that’s acceptable in your scenario).

Now, having covered these advanced concerns, we can move to practical exercises and examples to reinforce these concepts.

## Chapter 9: Practical Examples, Patterns, and Debugging Tips

In this chapter, we’ll go through some practical code patterns that incorporate everything we’ve discussed – concurrency, event loops, modules – and highlight common pitfalls and how to troubleshoot issues.

### 9.1 Practical Code Patterns

**Pattern: Producer-Consumer with Event Loop and Channel** – Suppose you have data being produced in Go (maybe from a Kafka consumer or some other stream) and you want to feed it into JS for processing. You can use a Go channel to buffer data and the event loop to pull from it:

```go
dataChan := make(chan string, 100)  // channel of work items

// Start a goroutine producing data:
go func() {
    for _, item := range []string{"one","two","three"} {
        dataChan <- item
        time.Sleep(100 * time.Millisecond)
    }
    close(dataChan)
}()

// JavaScript consumer function (pulls data and processes it)
vm := loop.Runtime()
vm.Set("pullData", func(call goja.FunctionCall) goja.Value {
    // This will be called on the event loop thread
    val, ok := <-dataChan
    if !ok {
        return goja.Null()  // signal no more data
    }
    return vm.ToValue(val)
})
```

Then in JS:

```javascript
while(true) {
    let item = pullData();
    if(item === null) break;
    console.log("Got item", item);
}
```

This loop, however, is running on the event loop thread – if the channel is empty, `pullData` would block the whole event loop (bad!). To avoid blocking in JS, a better approach is to use a non-blocking channel receive or a callback:

Better:

```go
vm.Set("tryPullData", func(call goja.FunctionCall) goja.Value {
    select {
    case val, ok := <-dataChan:
        if !ok {
            return goja.Null()
        }
        return vm.ToValue(val)
    default:
        return goja.Undefined()  // indicate no data available right now
    }
})
```

Then JS could do:

```javascript
function poll() {
    let item;
    while((item = tryPullData()) !== undefined) {
        if(item === null) {
            console.log("Stream ended");
            return;
        }
        console.log("Got", item);
    }
    // If undefined, no data now, schedule next poll
    setTimeout(poll, 100);
}
poll();
```

This pattern ensures the event loop isn’t stuck waiting on a channel – it checks quickly and if none, uses a timer to check later. This is a cooperative polling approach. Alternatively, you could invert it: when data arrives in Go, you could schedule a JS function via `RunOnLoop` to handle it (pushing rather than polling). That might be more efficient:

```go
go func() {
    for val := range dataChan {
        // For each item, schedule a JS callback
        loop.RunOnLoop(func(vm *goja.Runtime) {
            // assuming a JS global onData exists
            vm.RunString(fmt.Sprintf("onData(%q);", val))
        })
    }
}()
```

Then JS would have:

```javascript
function onData(item) {
    console.log("Got", item);
}
```

This push model is often better, but beware if data comes in faster than JS can handle, you might queue a lot of tasks. The channel buffer and back-pressure logic in Go should be tuned accordingly (if the channel is bounded, the producer will block when full, which is a natural back-pressure).

**Pattern: JS Callback for Go events** – Similar to above, but more formal:
If your application needs JS to handle some events or notifications (like a user-defined hook on a Go event), you can allow JS to register a callback, e.g.:

```go
var onEvent goja.Callable

vm.Set("registerCallback", func(call goja.FunctionCall) goja.Value {
    cb, ok := goja.AssertFunction(call.Argument(0))
    if !ok {
        panic(vm.ToValue("not a function"))
    }
    onEvent = cb
    return goja.Undefined()
})
```

Now when something happens in Go:

```go
if onEvent != nil {
    loop.RunOnLoop(func(vm *goja.Runtime) {
        _, err := onEvent(nil, vm.ToValue("someEventData"))
        if err != nil {
            // handle exception from callback
            fmt.Println("Callback error:", err)
        }
    })
}
```

JS usage:

```javascript
registerCallback(function(data) {
    console.log("Event received:", data);
});
```

This demonstrates storing a JS function (`onEvent`) and invoking it later on the event loop. We ensure thread safety by calling it via `RunOnLoop`. We also capture any error from the call (maybe the JS threw) and log it.

**Pattern: Long-running JS with Periodic Yields** – If you have JS code that might run for a long time (like a large loop), and you want to avoid starving other tasks (since it's single-threaded), you can cooperatively yield to the event loop by splitting the work. For example:

```javascript
function longTask(items) {
    return new Promise(resolve => {
        let i = 0;
        function processNext() {
            let start = Date.now();
            while(i < items.length && Date.now() - start < 50) {  // do 50ms of work
                doWork(items[i]);
                i++;
            }
            if(i < items.length) {
                // not done, yield for a moment
                setTimeout(processNext, 0);
            } else {
                resolve();
            }
        }
        processNext();
    });
}
```

This is a pure JS solution, breaking work in chunks and using `setTimeout` 0 to allow other events between chunks. This is similar to how one might avoid blocking the UI thread in a browser.

### 9.2 Debugging Tips

* **Logging and Print Statements:** The simplest debugging method is liberally using `fmt.Println` in Go and `console.log` in JS. Because Goja integrates easily with Go, you can print internal states at the boundaries. For example, print each time you enqueue a task with `RunOnLoop` and each time a task runs. This can help trace the flow of async events.
* **Use the Race Detector:** If you suspect a race, run your Go program with `-race`. It can catch some mistakes like accidentally accessing the runtime from two goroutines. Many times, as soon as you try the unsafe thing, Goja will panic (like the earlier example of concurrent access panicked with an index error), but the race detector might catch more subtle cases (like two goroutines accessing a shared Go object that JS also touches).
* **Step-by-Step Execution:** You can simulate stepping by sprinkling logs, or even by running the JS in smaller chunks. For instance, if something goes wrong deep in an async function, you can instrument that function to log its steps or inputs.
* **Examine the JS Stack:** If you catch an exception in Go (a `*Exception`), you can do `fmt.Println(exception.String())` which usually includes the JS stack trace. This is extremely useful to identify where in JS something went wrong. For example:

  ```go
  _, err := vm.RunString(`some buggy code`)
  if err != nil {
      if jserr, ok := err.(*goja.Exception); ok {
          fmt.Println("JS Exception:", jserr.Value().String())
      }
  }
  ```

  The output might show something like `ReferenceError: x is not defined at myFunc (<anonymous>:3:5)`, which tells you the file (or eval code) and line.
* **Timeouts and Infinite Loops:** If you suspect an infinite loop in JS is hanging the event loop, you can use `Runtime.Interrupt()` from another goroutine as a crude way to break it. For example, start a timer in Go that after 5 seconds calls `vm.Interrupt("timed out")`. If the JS is still running, it will throw an \*InterruptedError inside the JS code, unwinding it. You can catch that in Go as an error containing "timed out". This is a last resort, but important for not letting runaway scripts hang your host system. Remember to maybe catch that in JS if you want (you could catch the exception "timed out"). Usually, you won't allow arbitrary untrusted infinite loops, but if running untrusted code, this is a safety mechanism.
* **Fuzz Testing JS APIs:** If you expose some functions to JS, consider writing some random or edge-case tests in JS to see if you can break it. Also, unit test your Go module functions thoroughly – feed them unexpected inputs from JS (like weird types or huge values) to see how they behave.
* **Memory Profiling:** If memory usage grows, consider whether you have references in JS that keep data alive. Typical memory leaks could be leaving timers running (intervals not cleared), or global variables that accumulate. Use Go’s profiling tools to see if the Go heap is growing unexpectedly (though JS objects live in Go’s heap as part of goja’s allocations). If a lot of data is stuck in the JS VM and not freed, maybe references exist that prevent GC. You can force a GC in Goja by doing something like `runtime.RunScript("forceGC", "gc();")` if `gc` were exposed (note: not sure if goja has a manual GC trigger accessible; it's not standard JS to manually trigger GC).

### 9.3 Common Pitfalls Recap

Let's list out the common pitfalls one more time with quick advice:

* **Pitfall:** Calling JS from multiple goroutines – **Solution:** Use one event loop (or one runtime per goroutine) and `RunOnLoop` for cross-thread calls.
* **Pitfall:** Forgetting to resolve/reject on loop – you'll get panics or weird behavior – **Solution:** Always schedule promise resolution on the loop thread.
* **Pitfall:** Leaking goroutines or loops – e.g., starting an event loop and not stopping it, or launching goroutines that wait on something that never happens – **Solution:** Ensure proper Stop/Terminate calls on loops, and design goroutines to exit (check context or closed channels).
* **Pitfall:** Shared registry causing data race – **Solution:** If using multiple runtimes, prefer separate registries or ensure modules are safe.
* **Pitfall:** JS code is slow – **Solution:** Consider moving heavy parts to Go or break JS work into chunks, or run parallel VMs for parallel tasks.
* **Pitfall:** Not handling JS exceptions – they propagate to Go and might crash if uncaught – **Solution:** Wrap `vm.RunString` in error handling, use try/catch in JS around risky calls.
* **Pitfall:** Overuse of `eval`/`RunString` building strings – injecting values by string concatenation (like our onEvent example `vm.RunString(fmt.Sprintf("onData(%q)", val))`) is convenient but potentially risky if the string isn't properly escaped or if content is large. Prefer calling a stored callback if possible (we could have stored a `onData` function reference similarly to registerCallback).
* **Pitfall:** Complex data exchange using Export/ExportTo – conversion might not produce what you expect (e.g., Exporting a JS Date gives a time.Time or string?). Always test how data maps between Go and JS. Use custom logic if needed (like manually convert certain types).

By anticipating these pitfalls, you can avoid many frustrating bugs.

## Chapter 10: Exercises and Sample Projects

To solidify understanding, here are some exercises and project ideas:

**Exercise 1: Promise and Event Loop Basics** – Write a small program that uses Goja to print "Hello, World!" after 1 second using a JavaScript `setTimeout` simulation. (Hint: use `loop.SetTimeout` to schedule a `console.log` of "Hello, World!" after 1 second).

*What to learn:* Using the event loop and timers.

**Exercise 2: Async/Await Workflow** – Create a Goja script that defines an async function which waits on two promises (perhaps one resolves via `setTimeout`, another via an external Go function like a simulated I/O). Ensure that the function prints messages before, during, and after the awaits to demonstrate the execution order. Run this script through the event loop in Go.

*What to learn:* How `await` yields to other tasks and the necessity of the event loop to complete promises.

**Exercise 3: Native Module Implementation** – Implement a native module "time" that provides:

* `now()` returning current timestamp,
* `sleep(ms)` returning a Promise that resolves after given milliseconds (this can reuse a combination of Go `time.AfterFunc` or just `SetTimeout` inside the module loader).

Then in JS, use:

```javascript
const time = require('time');
console.log("Start:", time.now());
await time.sleep(1000);
console.log("End:", time.now());
```

Confirm that \~1000ms elapsed.

*What to learn:* Registering and using a custom native module, and combining it with promises.

**Exercise 4: Concurrent Data Processing** – Given an array of numbers in JS, use Go concurrency to compute the square of each number concurrently and return the results to JS. Specifically:

* Expose a function `parallelSquare(numbers)` in a module, which splits the input array into chunks (maybe 4 chunks), uses 4 goroutines to square each number in its chunk (in Go), then returns a Promise that resolves to the full array of squared numbers in order.
* You will need to coordinate combining the results and resolving the promise.

Test it with a large array and verify results.

*What to learn:* Combining Go concurrency with JS promises and ensuring ordering of results.

**Exercise 5: Error Handling Scenario** – Write a JS function that intentionally throws an error inside a `setTimeout` callback. For example:

```javascript
setTimeout(() => { throw new Error("TestError") }, 100);
```

Run this in the event loop. Observe what happens (probably the loop will panic because the error is uncaught). Then implement a mechanism to catch such errors so that the program doesn’t crash. This could be a global `process.on('uncaughtException')` simulation or wrapping callbacks with try/catch and logging.

*What to learn:* Importance of catching errors in async tasks and strategies to handle them.

**Project Idea 1: JS Plugin System** – Build a small application that loads user-provided JavaScript files (like plugins) and executes a known function from them (for example, each plugin exports a function `run(context)` that you call from Go with some context object). Use Goja to load these plugins safely (maybe sandboxing them by not providing dangerous modules). Manage concurrency such that each plugin runs isolated (maybe each in its own runtime or sequentially). This mimics how an app can be extended via scripting.

**Project Idea 2: Web Server with JS Handlers** – Use Goja to allow writing HTTP request handlers in JavaScript. For instance, your Go program uses `net/http`, and for certain routes, it delegates to a JS function to produce a response. The JS could access an object representing the request (with methods to get query params, etc.), and return a response. Pay attention to concurrency: each request is a goroutine in Go’s HTTP server, so you must decide to either queue these to one JS runtime (one at a time) or spin up a new JS runtime per request (like PocketBase did). Implement a simple version of this and measure performance differences between single runtime vs multiple.

**Project Idea 3: Task Scheduler** – Implement a job scheduler in JS that allows scheduling jobs at certain times (like cron) using the event loop. The JS could maintain a list of tasks with timestamps and the Go side could tick the clock. This is more of a logic exercise using the event loop (since `SetTimeout` could suffice for scheduling, the challenge is maybe to load tasks from an external source or allow dynamically adding tasks).

These exercises and projects will give hands-on experience and surface practical considerations not immediately obvious in theory.

## Chapter 11: Best Practices and Design Patterns for Goja Concurrency

In this final chapter, we summarize best practices and highlight key design patterns that have emerged throughout this book. Following these guidelines will help you build reliable, maintainable, and efficient concurrent applications with Goja.

### 11.1 Best Practices Summary

* **Keep Goja runtime usage single-threaded:** Only use one goroutine at a time to execute JS on a given `Runtime`. Use the event loop or channels to synchronize access. This avoids data races and crashes.
* **Use the `eventloop` for async tasks:** It provides a safe, structured way to simulate the JS event loop, with utilities for timers and scheduling. This greatly simplifies concurrency management compared to crafting your own loops.
* **Plan the concurrency model upfront:** Decide between one runtime (with queued tasks) vs. multiple runtimes (parallel execution) based on your needs. If you need to share state, one runtime is simpler. If you need parallelism, multiple runtimes can be used but require more careful state partitioning or synchronization.
* **Prefer promises (and `async/await`) over callback hell:** They lead to more readable code and integrate well with Goja’s capabilities. Now that Goja supports `async/await`, use it to avoid deeply nested callbacks. Be mindful of promise rejection handling (always `.catch` errors or use try/catch in async functions).
* **Integrate with Go’s context cancellation:** If your host app has contexts (e.g., HTTP request contexts), you might propagate cancellation to JS. For example, stop the event loop or mark a flag that JS checks. This prevents orphaned operations if the user disconnects or the app is shutting down.
* **Minimize crossing the Go-JS boundary in tight loops:** Each call from JS into a Go function or vice versa has overhead. If performance is an issue, do more work per call. For example, one function that returns a batch of data instead of many functions each returning one item.
* **Use `ExportTo` and `AssertFunction` for calling JS from Go:** This gives a clean, direct way to call a JS function with Go arguments, as opposed to constructing parameter strings. It also catches type mismatches early (if the value isn’t a function).
* **Be cautious with long-running JS code:** Because it will block the event loop. Break up large work or offload it. For truly CPU-heavy tasks, consider if Go concurrency or even parallel Goja runtimes would do better.
* **Secure the environment if running untrusted code:** Goja does not inherently sandbox beyond what you expose. So if running third-party scripts, only expose the APIs you want. For instance, don’t expose `os.Exit` or dangerous system calls. The `require` system will by default allow file reads of modules – if that’s an issue, override `SourceLoader` to control where scripts can be loaded from.
* **Memory management awareness:** Watch for closures or global variables in JS that accumulate state. Also, avoid retaining Goja `Value` or `Object` references in Go for a long time – keep them within the event loop context if possible (or document why it’s safe if you do).
* **Testing in isolation:** Test your JS code and Go integration code separately. Write pure JS tests for logic (maybe using Goja in a test to run a JS snippet), and pure Go tests for your module loaders, etc. Then integration tests for everything together.

### 11.2 Concurrency Design Patterns

* **Single Event Loop Hub Pattern:** One event loop receives events from many goroutines (via `RunOnLoop`). Good for applications where ordering needs to be preserved or where shared state in JS is central. E.g., a game engine where multiple Go subsystems (AI, physics) send events to be processed by game logic in JS on one thread.
* **Worker Pool of Runtimes:** Have a set of Goja runtimes pre-initialized (with required modules). When a task comes in, assign it to a runtime that isn’t busy (or the least busy). This is like a thread pool but with JS engines. It can be useful in a server handling user scripts concurrently. Just ensure isolation (each runtime has its own global state; you might clone initial state or reinitialize between tasks).
* **Pub-Sub with Event Loop:** Use the event loop as a central dispatcher for events to JS listeners. For example, you maintain a map of event name -> array of JS callback functions. When an event occurs (from Go), you use `RunOnLoop` to iterate through listeners and call them in JS. This pattern naturally emerges if your application needs something like Node’s `EventEmitter` in the JS environment.
* **Promise-based API wrapping:** Create promise-returning JS functions for any async Go operation. Internally, use `NewPromise` and `RunOnLoop` to resolve it. This standardizes your JS API – everything asynchronous returns a promise, making it easy to compose with `await`.
* **Stateful Modules vs Pure Functions:** Decide if a module will carry state (like a DB connection or in-memory cache) or just provide pure utility functions. Stateful modules might need an explicit `init()` and `shutdown()` in JS or be tied to lifecycle events in Go (for example, cleaning up on program exit).
* **Graceful Shutdown Pattern:** If your program is shutting down (e.g., server stopped), you might want to notify JS to clean up (maybe flush data, close resources). You can design a special function or event for this. For example, expose a `global.onShutdown` callback that scripts can assign to do cleanup. Then in Go, when shutting down, call that via event loop and give it a short time to finish. Also ensure to `Terminate()` the loop after that to kill any stray timers or goroutines.

### 11.3 Final Thoughts

Concurrency in Goja is powerful when used correctly: it allows you to script high-level logic while leveraging Go’s concurrency for performance. By respecting the single-threaded nature of the JS runtime and using the tools provided (`eventloop`, promises, etc.), you can avoid most pitfalls.

As a senior engineer, you have seen how critical it is to manage the interaction between two different concurrency models (Go’s and JavaScript’s). By combining them judiciously, you can get the best of both worlds: Go’s speed and scalability with JavaScript’s flexibility and ubiquity.

Always keep learning from real-world usage; libraries like Goja are often used in complex systems (like the PocketBase example and others). Community forums, GitHub issues, and updates to Goja can provide new insights or features (for instance, if in the future Goja adds true multi-threading via SharedArrayBuffer or similar – unlikely in pure Go, but one never knows). Stay updated with Goja’s repository and release notes for any changes in concurrency support or new modules in goja\_nodejs.

In closing, the key to success is to **think in terms of events and messages** when dealing with JavaScript in Go. If you design your system such that components communicate by sending events or scheduling tasks (rather than shared memory), you will naturally align with the event loop model and avoid many concurrency issues. This is essentially the actor model or message-passing concurrency, which scales well from a single thread (JS) to multi-thread (Go, with channels) architecture.

Happy coding with Goja and may your asynchronous code be bug-free and performant!
