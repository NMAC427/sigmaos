CXX = g++
CXXFLAGS = -std=c++20 -fPIC -shared -Wall -Wextra -O2

TARGET = ld_preload.so
SOURCE = ld_preload.cpp

all: $(TARGET)

$(TARGET): $(SOURCE)
	$(CXX) $(CXXFLAGS) -o $@ $< -ldl

clean:
	rm -f $(TARGET)

.PHONY: all clean