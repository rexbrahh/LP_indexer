#include "candle_worker.hpp"
#include "fixed_point.hpp"
#include "publisher.hpp"

#include <chrono>
#include <fstream>
#include <iostream>
#include <sstream>
#include <string>
#include <thread>

namespace {

void usage() {
  std::cerr << "Usage: candle_replay --input FILE [--nats-url URL] [--stream NAME] "
            << "[--subject-root ROOT] [--sleep-sec N]\n";
}

bool parse_args(int argc, char **argv, std::string &input_path,
                std::string &nats_url, std::string &stream,
                std::string &subject_root, int &sleep_sec) {
  for (int i = 1; i < argc; ++i) {
    std::string arg(argv[i]);
    if (arg == "--input" && i + 1 < argc) {
      input_path = argv[++i];
    } else if (arg == "--nats-url" && i + 1 < argc) {
      nats_url = argv[++i];
    } else if (arg == "--stream" && i + 1 < argc) {
      stream = argv[++i];
    } else if (arg == "--subject-root" && i + 1 < argc) {
      subject_root = argv[++i];
    } else if (arg == "--sleep-sec" && i + 1 < argc) {
      sleep_sec = std::stoi(argv[++i]);
    } else if (arg == "--help") {
      usage();
      return false;
    } else {
      std::cerr << "unknown argument: " << arg << "\n";
      usage();
      return false;
    }
  }
  if (input_path.empty()) {
    usage();
    return false;
  }
  return true;
}

candle::FixedPrice to_fixed(double value) {
  return candle::FixedPoint::from_double(value).raw();
}

} // namespace

int main(int argc, char **argv) {
  std::string input_path;
  std::string nats_url = "nats://127.0.0.1:4222";
  std::string stream = "DEX";
  std::string subject_root = "dex.sol";
  int sleep_sec = 2;

  if (!parse_args(argc, argv, input_path, nats_url, stream, subject_root,
                  sleep_sec)) {
    return 1;
  }

  std::ifstream file(input_path);
  if (!file.is_open()) {
    std::cerr << "failed to open input file: " << input_path << "\n";
    return 1;
  }

  candle::CandleWorker worker;
  candle::JetStreamConfig js_cfg;
  js_cfg.url = nats_url;
  js_cfg.stream = stream;
  js_cfg.subject_root = subject_root;

  try {
    worker.set_jetstream_publisher(js_cfg);
  } catch (const std::exception &ex) {
    std::cerr << "failed to initialize JetStream publisher: " << ex.what()
              << "\n";
    return 1;
  }

  worker.start();

  std::string line;
  std::size_t count = 0;
  while (std::getline(file, line)) {
    if (line.empty() || line[0] == '#') {
      continue;
    }
    std::stringstream ss(line);
    std::string pair_id;
    std::string ts_str;
    std::string price_str;
    std::string base_str;
    std::string quote_str;

    if (!std::getline(ss, pair_id, ',')) {
      continue;
    }
    if (!std::getline(ss, ts_str, ',')) {
      std::cerr << "missing timestamp in line: " << line << "\n";
      continue;
    }
    if (!std::getline(ss, price_str, ',')) {
      std::cerr << "missing price in line: " << line << "\n";
      continue;
    }
    if (!std::getline(ss, base_str, ',')) {
      std::cerr << "missing base amount in line: " << line << "\n";
      continue;
    }
    if (!std::getline(ss, quote_str, ',')) {
      std::cerr << "missing quote amount in line: " << line << "\n";
      continue;
    }

    try {
      uint64_t timestamp = std::stoull(ts_str);
      double price = std::stod(price_str);
      double base = std::stod(base_str);
      double quote = std::stod(quote_str);

      worker.on_trade(pair_id, timestamp, to_fixed(price), to_fixed(base),
                      to_fixed(quote));
      ++count;
    } catch (const std::exception &ex) {
      std::cerr << "failed to parse line: " << line << " error: " << ex.what()
                << "\n";
    }
  }

  if (count == 0) {
    std::cerr << "no trades processed from input" << std::endl;
  } else {
    std::cout << "processed " << count << " trades" << std::endl;
  }

  std::this_thread::sleep_for(std::chrono::seconds(sleep_sec));
  worker.stop();
  return 0;
}
