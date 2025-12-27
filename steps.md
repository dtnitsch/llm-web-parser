### Project Setup & Structure
*   Initialize a new Go module using `go mod init [your-project-name]`.
*   Establish a clean project layout to separate concerns:
    *   `/cmd/worker`: Main entry point for the application.
    *   `/pkg/fetcher`: Code to download web content.
    *   `/pkg/parser`: Logic for stripping HTML/JS and extracting text.
    *   `/pkg/storage`: Functions for saving artifacts to disk.
    *   `/pkg/analytics`: Your analytics functions (e.g., word count).
    *   `/pkg/mapreduce`: The core MapReduce framework logic.
    *   `/configs`: For configuration files (e.g., YAML or JSON).

### 1. Web Fetching & Stripping
*   **Implement a Robust Fetcher:**
    *   Use Go's standard `net/http` package to create a reusable HTTP client.
    *   Crucially, set connection and request timeouts on the `http.Client` to handle unresponsive sites and prevent indefinite hangs. Use `context` for request cancellation.
    *   Build a concurrent worker pool using goroutines and channels to fetch multiple URLs in parallel.
*   **Create a Content Parser/Stripper:**
    *   Evaluate and choose a third-party HTML parsing library. `golang.org/x/net/html` is standard, but `go-query` is more powerful and often easier for this task.
    *   Write functions to traverse the parsed HTML and extract only the relevant text content, discarding `<script>` and `<style>` tags.
*   **Implement Artifact Storage:**
    *   Create a storage service that saves the raw HTML and the stripped text content to a designated artifacts directory.
    *   Use a consistent naming convention for saved files, such as hashing the URL, to avoid filesystem issues and allow for easy retrieval.

### 2. Analytics Pipeline
*   **Define an Analytics Interface:**
    *   Create a simple Go interface for an "analytic" that can be run on text data. This allows you to easily add new analytics in the future.
*   **Implement Initial Analytics:**
    *   Start with a concrete and useful analytic, such as keyword frequency counting (term frequency).
    *   This function should take a string of text and return a `map[string]int` representing the word counts. This will serve as a perfect first use case for your MapReduce implementation.

### 3. MapReduce Framework
*   **Define Core MapReduce Functions:**
    *   Define the function signatures for your `Map` and `Reduce` operations. A common pattern is:
        *   `Map(input string) []KeyValue`
        *   `Reduce(key string, values []int) int` (for a word count example)
*   **Build the MapReduce Coordinator:**
    *   **Map Stage:** Write a function that takes a list of text inputs (from your stripped web pages) and uses a worker pool of goroutines to apply the `Map` function to each input concurrently.
    *   **Shuffle Stage:** After the mappers are done, collect all the intermediate key-value pairs. Create a single data structure (like a `map[string][]int`) that groups all values by their key.
    *   **Reduce Stage:** Use another worker pool of goroutines to run the `Reduce` function on each key and its list of associated values.
    *   The coordinator should manage these three stages in sequence and return the final aggregated results.

### Project Finalization
*   **Externalize Configuration:**
    *   Do not hardcode values like timeouts, worker pool sizes, or artifact paths. Load them from a configuration file in your `/configs` directory.
*   **Write Unit Tests:**
    *   Write tests for each package, especially the parser, analytics functions, and the MapReduce logic, to ensure correctness and prevent regressions.