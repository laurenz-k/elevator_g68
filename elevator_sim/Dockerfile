FROM ubuntu

RUN apt-get update && apt-get install -y libc6

COPY ./SimElevatorServer /src/SimElevatorServer
RUN chmod +x /src/SimElevatorServer
EXPOSE 15657

ENTRYPOINT ["./src/SimElevatorServer"]
