## ./autoscaler
A work-in-progress, but is fully functional. Only dependencies are Docker and cgroupv2. If you want to develop and build locally, you'll need Go 1.21.9, BPF enabled and possibly other things (I'm yet to write a list).


Works with vanilla Docker Swarm out of the box. Setup using the `docker swarm init` and `join` cmds.

Look at `./autoscaler/build/docker-compose.yml` and `./autoscaler/build/config.yaml` for examples.

Will scale up and down based on CPU/memory utilisation. When scaled to zero, will use BPF to wake back up and scale to 1.

## ./infra
Various scripts used to set everything up while I've been developing.

## ./archive
Stuff I don't want to delete just yet.
