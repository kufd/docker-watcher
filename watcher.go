package main

import (
    "fmt"
    "time"
    "github.com/kufd/docker-watcher/watcher"
    "log"
    "github.com/docker/docker/client"
)

func main() {
    log.Println("Docker watcher started.");

    imageLifetime, containerLifetime, keepImages, keepContainers, watchInterval := watcher.ParseArgs();

    defaultHeaders := map[string]string{"User-Agent": "engine-api-cli-1.0"}
    client, err := client.NewClient("unix:///var/run/docker.sock", "", nil, defaultHeaders)
    if err != nil {
        panic(err)
    }

    cycleCounter := 0;
    for {
        currentTime := time.Now().Unix()

        if cycleCounter == 0 {
            fmt.Println("");
            watcher.PrintDockerStatusReport(client);
        }
        fmt.Println("");
        watcher.RemoveOldContainers(client, keepContainers, containerLifetime, currentTime);
        fmt.Println("");
        watcher.RemoveOldImages(client, keepImages, imageLifetime, currentTime);
        fmt.Println("");
        watcher.PrintDockerStatusReport(client);

        cycleCounter++;

        time.Sleep(time.Duration(watchInterval) * time.Second);
    }

    log.Println("Docker watcher stopped.");
}
