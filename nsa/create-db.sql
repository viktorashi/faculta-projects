CREATE DATABASE IF NOT EXISTS `satelites`;
USE `satelites`;

CREATE TABLE IF NOT EXISTS `satelite` (
  `id` INT NOT NULL AUTO_INCREMENT,
  `name` TEXT NOT NULL,
  `semimajor_axis` DOUBLE NOT NULL CHECK (`semimajor_axis` > 6378),
  `eccentricity` DOUBLE NOT NULL CHECK (`eccentricity` >= 0.0 AND `eccentricity` < 1.0),
  `inclination` DOUBLE NOT NULL CHECK (`inclination` >= 0.0 AND `inclination` <= 180.0),
  `longitude_ascending_node` DOUBLE NOT NULL CHECK (`longitude_ascending_node` >= 0.0 AND `longitude_ascending_node` <= 360.0),
  `argument_of_perigee` DOUBLE NOT NULL CHECK (`argument_of_perigee` >= 0.0 AND `argument_of_perigee` <= 360.0),
  PRIMARY KEY (`id`)
) ENGINE = InnoDB;
