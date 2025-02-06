docker build -t elevator_sim .
docker run -it --rm -p 15657:15657 --name elevator_sim elevator_sim
