message(STATUS "Building cossim-srv-cpp proc")

add_executable(cossim-srv-cpp
  main.cc
)

target_link_libraries(cossim-srv-cpp PRIVATE 
  sigmaos_core
  spin_proto
  app_spin
  cossim_proto
  app_cossim
  cache_proto
  app_cache
)

target_include_directories(cossim-srv-cpp PRIVATE ${CMAKE_SOURCE_DIR})
