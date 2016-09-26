package main

import (
    "fmt"
    "time"
    "strings"
    "log"

    "github.com/docker/docker/client"
    "github.com/docker/docker/api/types"
    "golang.org/x/net/context"

    "github.com/docopt/docopt-go"
    "strconv"
)

func main() {
    log.Println("Docker watcher started.");

    imageLifetime, containerLifetime, keepImages, watchInterval := parseArgs();
    currentTime := time.Now().Unix()


    defaultHeaders := map[string]string{"User-Agent": "engine-api-cli-1.0"}
    client, err := client.NewClient("unix:///var/run/docker.sock", "", nil, defaultHeaders)
    if err != nil {
        panic(err)
    }

    for {
        fmt.Println("");
        removeOldContainers(client, containerLifetime, currentTime);
        fmt.Println("");
        removeOldImages(client, keepImages, imageLifetime, currentTime);

        time.Sleep(time.Duration(watchInterval) * time.Second);
    }

    log.Println("Docker watcher stopped.");
}

func parseArgs() (imageLifetime int64, containerLifetime int64, keepImages []string, watchInterval int64) {
    usage := `Docker Watcher.

    Usage:
      watcher [--keepImage=<image name>]... [--imageLifetime=<seconds>] [--containerLifetime=<seconds>] [--watchInterval=<seconds>]
      watcher --version


    Options:
      -h --help     Show this screen.
      -v --version     Show version.
      --keepImage=<image name>  Image to keep on host.
      --imageLifetime=<seconds>  Image lifetime [default: 259200](3 days).
      --containerLifetime=<seconds>  Container lifetime [default: 259200](3 days).
      --watchInterval=<seconds>  Pause duration between checking docker status [default: 600](10 minutes).`

    programmArguments, err := docopt.Parse(usage, nil, true, "Docker Watcher 0.0.2", false)
    if err != nil {
        panic(err)
    }

    imageLifetime, err = strconv.ParseInt(programmArguments["--imageLifetime"].(string), 10, 64)
    if err != nil {
        panic(err)
    }
    containerLifetime, err = strconv.ParseInt(programmArguments["--containerLifetime"].(string), 10, 64)
    if err != nil {
        panic(err)
    }
    watchInterval, err = strconv.ParseInt(programmArguments["--watchInterval"].(string), 10, 64)
    if err != nil {
        panic(err)
    }

    keepImages, _ = programmArguments["--keepImages"].([]string);

    log.Printf("Parameters:\n imageLifetime: %v \n containerLifetime: %v \n keepImages: %v \n watchInterval: %v", imageLifetime, containerLifetime, keepImages, watchInterval);

    return imageLifetime, containerLifetime, keepImages, watchInterval;
}

func removeOldImages(client *client.Client, keepImages []string, imageLifetime int64, currentTime int64)  {

    var imagesToRemove []types.Image;
    imagesRemoved := make(map[string]string);
    images := getAllImages(client);
    containers := getAllContainers(client);

    log.Println("Going to remove unused images with their child images.");

    for _, image := range images {
        remove := false;

        //remove images by lifetime
        if currentTime > image.Created + imageLifetime {
            remove = true;
        }

        //keep images which are arent images for other images
        if isParentImage(image, images) {
            remove = false;
        }

        //keep images from keepImages list
        for _, repoTag := range image.RepoTags {
            for _, keepImage := range keepImages {
                if repoTag != keepImage && keepImage != repoTag[:strings.Index(repoTag, ":")] {
                    remove = false;
                }
            }
        }

        //keep images which have existed containers
        if isContainerFromImageExists(image, containers) {
            remove = false;
        }

        if remove {
            imagesToRemove = append(imagesToRemove, image)
        }
    }

    if len(imagesToRemove) > 0 {
        for _, image := range imagesToRemove {
            _, found := imagesRemoved[image.ID]
            if !found {
                log.Println(image.RepoTags, " - ", image.ID, " ...");
                justImagesRemoved, err := client.ImageRemove(context.Background(), image.ID, types.ImageRemoveOptions{Force: true, PruneChildren: true});
                if err != nil {
                    panic(err)
                }

                for _, justImageRemoves := range justImagesRemoved {
                    imagesRemoved[justImageRemoves.Deleted] = justImageRemoves.Deleted;
                }

                log.Println("REMOVED");
            }
        }
    } else {
        log.Println("No images to remove found.");
    }

    log.Println("Images removing finished.");
}

func removeOldContainers(client *client.Client, containerLifetime int64, currentTime int64)  {
    containers := getAllContainers(client);
    var containersToRemove []types.Container;

    log.Println("Going to remove unused containers.");
    for _, container := range containers {
        if "Exited" == container.Status[0:6] {
            containerInfo, err := client.ContainerInspect(context.Background(), container.ID);
            if err != nil {
                panic(err)
            }

            finishedAt, err := time.Parse(time.RFC3339Nano, containerInfo.State.FinishedAt)
            if err != nil {
                panic(err)
            }

            if finishedAt.Unix() + containerLifetime < currentTime {
                containersToRemove = append(containersToRemove, container)
            }
        }
    }

    if (len(containersToRemove) > 0) {
        for _, container := range containersToRemove {
            log.Println(container.Names, " - ", container.ID, " ...");
            err := client.ContainerRemove(context.Background(), container.ID, types.ContainerRemoveOptions{RemoveVolumes: true, Force: true});
            if err != nil {
                panic(err)
            }
            log.Println("REMOVED");
        }
    } else {
        log.Println("No containers to remove found.");
    }

    log.Println("Containers removing finished.");
}

func isParentImage(image types.Image, images []types.Image) bool {
    result := false;

    for _, imageFromList := range images {
        if (image.ID == imageFromList.ParentID) {
            result = true;
            break;
        }
    }

    return result;
}

func isContainerFromImageExists(image types.Image, containers []types.Container) bool {
    result := false;

    for _, container := range containers {
        if (image.ID == container.ImageID) {
            result = true;
            break;
        }
    }

    return result;
}

func getAllContainers(client  *client.Client) []types.Container {
    containerListOptions := types.ContainerListOptions{All: true}
    containers, err := client.ContainerList(context.Background(), containerListOptions)
    if err != nil {
        panic(err)
    }

    return containers;
}

func getAllImages(client  *client.Client) []types.Image {
    imageListOptions := types.ImageListOptions{All: true}
    images, err := client.ImageList(context.Background(), imageListOptions)
    if err != nil {
        panic(err)
    }

    return images;
}

