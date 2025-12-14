#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <sys/un.h>
#include <time.h>
#include <arpa/inet.h>
#include <signal.h>
#include <math.h>

FILE *output_file;
long *durations;

void error(const char *msg) {
    perror(msg);
    exit(1);
}

void handle_sigint(int sig) {
    printf("Caught signal %d, cleaning up and exiting...\n", sig);
    if (durations != NULL) free(durations);
    if (output_file != NULL) fclose(output_file);
    exit(0);
}

double calculate_mean(long *data, int size) {
    double sum = 0.0;
    for (int i = 0; i < size; i++) {
        sum += data[i];
    }
    return sum / size;
}

double calculate_standard_deviation(long *data, int size, double mean) {
    double sum = 0.0;
    for (int i = 0; i < size; i++) {
        sum += pow(data[i] - mean, 2);
    }
    return sqrt(sum / size);
}

void check_confidence_interval(long *data, int size) {
    double mean = calculate_mean(data, size);
    double stddev = calculate_standard_deviation(data, size, mean);
    double margin_of_error = 1.96 * (stddev / sqrt(size)); // 95% confidence interval

    double lower_bound = mean - margin_of_error;
    double upper_bound = mean + margin_of_error;

    printf("Mean: %lf, Standard Deviation: %lf\n", mean, stddev);
    printf("95%% Confidence Interval: [%lf, %lf]\n", lower_bound, upper_bound);
    printf("95%% Confidence Interval: [%.2lf%%, %.2lf%%]\n", (lower_bound / mean) * 100, (upper_bound / mean) * 100);

    if (margin_of_error / mean > 0.025) {
        fprintf(stderr, "Warning: The margin of error exceeds 2.5%% of the mean.\n");
    }
}

int main(int argc, char *argv[]) {
    signal(SIGINT, handle_sigint);

    if (argc < 5) {
        fprintf(stderr, "Usage: %s protocol address count output_path\n", argv[0]);
        exit(1);
    }

    char *protocol = argv[1];
    char *address = argv[2];
    int count = atoi(argv[3]);
    char *output_path = argv[4];
    output_file = fopen(output_path, "w");
    if (output_file == NULL) error("ERROR opening output file");

    printf("Starting performance test: protocol=%s, address=%s, count=%d, output_path=%s\n", protocol, address, count, output_path);

    int sockfd;
    struct sockaddr_in serv_addr;
    struct sockaddr_un serv_addr_un;
    struct timespec start, end;
    durations = malloc(count * sizeof(long));
    if (durations == NULL) error("ERROR allocating memory");

    for (int i = 0; i < count; i++) {
        // printf("Iteration %d/%d\n", i + 1, count);
        if (strcmp(protocol, "TCP") == 0) {
            char address_copy[256];
            strncpy(address_copy, address, sizeof(address_copy) - 1);
            address_copy[sizeof(address_copy) - 1] = '\0';

            char *ip = strtok(address_copy, ":");
            char *port = strtok(NULL, ":");
            if (ip == NULL) {
                fprintf(stderr, "Invalid address format: missing IP address. Use XX.XX.XX.XX:port\n");
                exit(1);
            }
            if (port == NULL) {
                fprintf(stderr, "Invalid address format: missing port number. Use XX.XX.XX.XX:port\n");
                exit(1);
            }

            sockfd = socket(AF_INET, SOCK_STREAM, 0);
            if (sockfd < 0) error("ERROR opening socket");

            memset(&serv_addr, 0, sizeof(serv_addr));
            serv_addr.sin_family = AF_INET;
            serv_addr.sin_addr.s_addr = inet_addr(ip);
            serv_addr.sin_port = htons(atoi(port));

            // printf("Connecting to TCP server at %s:%s\n", ip, port);

            clock_gettime(CLOCK_MONOTONIC, &start);
            if (connect(sockfd, (struct sockaddr *) &serv_addr, sizeof(serv_addr)) < 0)
                error("ERROR connecting");
            close(sockfd);
            clock_gettime(CLOCK_MONOTONIC, &end);
        } else if (strcmp(protocol, "UNIX") == 0) {
            sockfd = socket(AF_UNIX, SOCK_STREAM, 0);
            if (sockfd < 0) error("ERROR opening socket");

            memset(&serv_addr_un, 0, sizeof(serv_addr_un));
            serv_addr_un.sun_family = AF_UNIX;
            strncpy(serv_addr_un.sun_path, address, sizeof(serv_addr_un.sun_path) - 1);

            // printf("Connecting to UNIX server at %s\n", address);

            clock_gettime(CLOCK_MONOTONIC, &start);
            if (connect(sockfd, (struct sockaddr *) &serv_addr_un, sizeof(serv_addr_un)) < 0)
                error("ERROR connecting");
            close(sockfd);
            clock_gettime(CLOCK_MONOTONIC, &end);
        } else {
            fprintf(stderr, "Unsupported protocol: %s\n", protocol);
            exit(1);
        }

        durations[i] = (end.tv_sec - start.tv_sec) * 1000000000L + (end.tv_nsec - start.tv_nsec);
        fprintf(output_file, "%ld\n", durations[i]); // 各イテレーションごとに結果を書き込む
    }

    printf("Performance test completed. Writing results to %s\n", output_path);

    free(durations);
    fclose(output_file);

    check_confidence_interval(durations, count);

    return 0;
}
