package watcher

import (
    "time"
    "strings"
    "log"
    "github.com/docker/docker/client"
    "github.com/docker/docker/api/types"
    "golang.org/x/net/context"
    "github.com/docopt/docopt-go"
    "strconv"

    units "github.com/docker/go-units"
)

func PrintDockerStatusReport(client *client.Client) {
    serverVersion, err := client.ServerVersion(context.Background())
    if err != nil {
        panic(err)
    }

    serverInfo, err := client.Info(context.Background())
    if err != nil {
        panic(err)
    }

    discUsage, err := client.DiskUsage(context.Background())
    if err != nil {
        panic(err)
    }

    report := `Docker status report:

    Docker version: %v
    Docker API version: %v
    Docker min API version: %v
    GO version: %v

    Total containers: %v
    Running containers: %v
    Paused containers: %v
    Stopped containers: %v

    Total images: %v

    Total size: %v
    `

    log.Printf(
        report,
        serverVersion.Version,
        serverVersion.APIVersion,
        serverVersion.MinAPIVersion,
        serverVersion.GoVersion,
        serverInfo.Containers,
        serverInfo.ContainersRunning,
        serverInfo.ContainersPaused,
        serverInfo.ContainersStopped,
        serverInfo.Images,
        units.HumanSize(float64(discUsage.LayersSize)));
}

func ParseArgs() (imageLifetime int64, containerLifetime int64, keepImages []string, keepContainers []string, watchInterval int64) {
    usage := `Docker Watcher.

    Usage:
      watcher [--keepImage=<image name>]... [--keepContainer=<container name>]... [--imageLifetime=<seconds>] [--containerLifetime=<seconds>] [--watchInterval=<seconds>]
      watcher --version


    Options:
      -h --help     Show this screen.
      -v --version     Show version.
      --keepImage=<image name>  Image to keep on host.
      --keepContainer=<container name>  Container to keep on host.
      --imageLifetime=<seconds>  Image lifetime [default: 259200](3 days).
      --containerLifetime=<seconds>  Container lifetime [default: 259200](3 days).
      --watchInterval=<seconds>  Pause duration between checking docker status [default: 3600](1 hour).`

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

    keepImages, _ = programmArguments["--keepImage"].([]string);
    keepContainers, _ = programmArguments["--keepContainer"].([]string);

    log.Printf("Parameters:\n imageLifetime: %v \n containerLifetime: %v \n keepImages: %v \n keepContainers: %v \n watchInterval: %v", imageLifetime, containerLifetime, keepImages, keepContainers, watchInterval);

    return imageLifetime, containerLifetime, keepImages, keepContainers, watchInterval;
}

func RemoveOldImages(client *client.Client, keepImages []string, imageLifetime int64, currentTime int64)  {

    var imagesToRemove []types.ImageSummary;
    imagesRemoved := make(map[string]string);
    images := getAllImages(client);

    log.Println("Going to remove unused images with their child images.");

    for _, image := range images {
        remove := false;

        //remove images by lifetime
        if currentTime > image.Created + imageLifetime {
            remove = true;
        }

        //keep images which are parent images for other images
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
        if image.Containers > 0 {
            remove = false;
        }

        if remove {
            imagesToRemove = append(imagesToRemove, image)
        }
    }

    if len(imagesToRemove) > 0 {
        counter := 0;
        for _, image := range imagesToRemove {
            _, found := imagesRemoved[image.ID]
            if !found {
                counter++;
                printImageInfo(image, counter);
                justImagesRemoved, err := client.ImageRemove(context.Background(), image.ID, types.ImageRemoveOptions{Force: true, PruneChildren: true});
                if err != nil {
                    panic(err)
                }

                for _, justImageRemoves := range justImagesRemoved {
                    imagesRemoved[justImageRemoves.Deleted] = justImageRemoves.Deleted;
                }
            }
        }
    } else {
        log.Println("No images to remove found.");
    }

    log.Println("Images removing finished.");
}

func RemoveOldContainers(client *client.Client, keepContainers  []string, containerLifetime int64, currentTime int64)  {
    containers := getAllContainers(client);
    var containersToRemove []types.Container;

    log.Println("Going to remove unused containers.");
    for _, container := range containers {
        remove := false;

        if "Exited" == container.Status[0:6] || "Created" == container.Status[0:7] {
            containerInfo, err := client.ContainerInspect(context.Background(), container.ID);
            if err != nil {
                panic(err)
            }

            finishedAt, err := time.Parse(time.RFC3339Nano, containerInfo.State.FinishedAt)
            if err != nil {
                panic(err)
            }

            if finishedAt.Unix() + containerLifetime < currentTime {
                remove = true;
            }
        }

        //do not remove container from keepContainers list
        for _, name := range container.Names {
            for _, keepContainer := range keepContainers {
                if  strings.Trim(name, "/") ==  strings.Trim(keepContainer, "/") {
                    remove = false;
                }
            }
        }

        if (remove) {
            containersToRemove = append(containersToRemove, container);
        }
    }

    if (len(containersToRemove) > 0) {
        counter := 0;
        for _, container := range containersToRemove {
            counter++;
            printContainerInfo(container, counter);
            err := client.ContainerRemove(context.Background(), container.ID, types.ContainerRemoveOptions{RemoveVolumes: true, Force: true});
            if err != nil {
                panic(err)
            }
        }
    } else {
        log.Println("No containers to remove found.");
    }

    log.Println("Containers removing finished.");
}

func printContainerInfo(container types.Container, containerNumber int) {
    containerInfo := "  id: " + container.ID + "\n";
    containerInfo += "  size: " + units.HumanSize(float64(container.SizeRw)) + "\n";
    containerInfo += "  names:\n";
    for _, name := range container.Names {
        containerInfo += "    - " + name + "\n";
    }

    log.Printf("Container #%v\n%v", containerNumber, containerInfo);
}

func printImageInfo(image types.ImageSummary, imageNumber int) {
    imageInfo := "  id: " + image.ID + "\n";
    imageInfo += "  size: " + units.HumanSize(float64(image.Size)) + "\n";
    imageInfo += "  tags:\n";
    for _, repoTag := range image.RepoTags {
        imageInfo += "    - " + repoTag + "\n";
    }

    log.Printf("Image #%v\n%v", imageNumber, imageInfo);
}

func isParentImage(image types.ImageSummary, images []types.ImageSummary) bool {
    result := false;

    for _, imageFromList := range images {
        if (image.ID == imageFromList.ParentID) {
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

func getAllImages(client  *client.Client) []types.ImageSummary {
    imageListOptions := types.ImageListOptions{All: true}
    images, err := client.ImageList(context.Background(), imageListOptions)
    if err != nil {
        panic(err)
    }

    return images;
}

